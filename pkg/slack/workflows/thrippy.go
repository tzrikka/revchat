package workflows

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/tzrikka/revchat/internal/logger"
	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/slack/activities"
	thrippypb "github.com/tzrikka/thrippy-api/thrippy/v1"
)

const (
	grpcTimeout = 3 * time.Second

	createLinkTimeout = 15 * time.Second

	pollLinkTimeout    = 2 * time.Second
	pollLinkInterval   = 1 * time.Second
	waitForLinkTimeout = 1 * time.Minute

	errLinkAuthzTimeout = "link not authorized yet" // For some reason [errors.Is] doesn't work across Temporal?
)

type linkData struct {
	ID    string `json:"id"`
	Nonce string `json:"nonce"`
}

// initThrippyLinks initializes the configuration for personal Thrippy links,
// based on the Thrippy link with the given ID. This is expected to be called
// once at worker startup, and is used by [Config.createThrippyLinkActivity].
func (c *Config) initThrippyLinks(ctx context.Context, id string) {
	conn, client, err := c.thrippyClient()
	if err != nil {
		logger.FatalContext(ctx, "failed to initialize Thrippy client", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	req := thrippypb.GetLinkRequest_builder{LinkId: new(id)}.Build()
	resp, err := client.GetLink(ctx, req)
	if err != nil {
		logger.FatalContext(ctx, fmt.Sprintf("failed to read the Thrippy link associated with the ID %q", id), err)
	}

	c.ThrippyLinksTemplate = resp.GetTemplate()
	c.thrippyLinksClientID = resp.GetOauthConfig().GetClientId()
	c.thrippyLinksClientSecret = resp.GetOauthConfig().GetClientSecret()
	logger.FromContext(ctx).Info("template for personal Thrippy links: " + c.ThrippyLinksTemplate)
}

// createThrippyLink executes [createThrippyLinkActivity] as a local activity.
func (c *Config) createThrippyLink(ctx workflow.Context) (string, string, error) {
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: createLinkTimeout,
	})

	ld := new(linkData)
	if err := workflow.ExecuteLocalActivity(ctx, c.createThrippyLinkActivity).Get(ctx, ld); err != nil {
		return "", "", err
	}

	return ld.ID, ld.Nonce, nil
}

// createThrippyLinkActivity creates a new Thrippy link to authorize a Bitbucket or GitHub user.
// The new link is based on the details associated with the ID in [Config.initThrippyLinks].
func (c *Config) createThrippyLinkActivity(ctx context.Context) (*linkData, error) {
	conn, client, err := c.thrippyClient()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	// Create the link to generate an ID.
	createReq := thrippypb.CreateLinkRequest_builder{
		Template: new(c.ThrippyLinksTemplate),
		OauthConfig: thrippypb.OAuthConfig_builder{
			ClientId:     new(c.thrippyLinksClientID),
			ClientSecret: new(c.thrippyLinksClientSecret),
		}.Build(),
	}.Build()

	createResp, err := client.CreateLink(ctx, createReq)
	if err != nil {
		return nil, err
	}

	// Retrieve the new link's configuration to get its nonce.
	getReq := thrippypb.GetLinkRequest_builder{LinkId: new(createResp.GetLinkId())}.Build()
	getResp, err := client.GetLink(ctx, getReq)
	if err != nil {
		return nil, err
	}

	return &linkData{ID: createResp.GetLinkId(), Nonce: getResp.GetOauthConfig().GetNonce()}, nil
}

// waitForThrippyLinkCreds waits for the user to complete
// the OAuth flow for the Thrippy link with the given ID.
func (c *Config) waitForThrippyLinkCreds(ctx workflow.Context, linkID string) error {
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: waitForLinkTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    pollLinkInterval,
			BackoffCoefficient: 1.0,
		},
	})

	return workflow.ExecuteLocalActivity(ctx, c.pollThrippyLinkActivity, linkID).Get(ctx, nil)
}

func (c *Config) pollThrippyLinkActivity(ctx context.Context, linkID string) error {
	conn, client, err := c.thrippyClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, pollLinkTimeout)
	defer cancel()

	req := thrippypb.GetCredentialsRequest_builder{LinkId: new(linkID)}.Build()
	resp, err := client.GetCredentials(ctx, req)
	if err != nil {
		return err
	}

	if len(resp.GetCredentials()) == 0 {
		return errors.New(errLinkAuthzTimeout)
	}

	return nil
}

// deleteThrippyLink deletes the Thrippy link with the given ID. This is a
// best-effort cleanup in case the user opted-in but didn't authorize us in time.
func (c *Config) deleteThrippyLink(ctx workflow.Context, linkID string) error {
	conn, client, err := c.thrippyClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	grpcCtx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
	defer cancel()

	req := thrippypb.DeleteLinkRequest_builder{LinkId: new(linkID)}.Build()
	if _, err = client.DeleteLink(grpcCtx, req); err != nil {
		logger.From(ctx).Error("failed to delete Thrippy link", slog.Any("error", err), slog.String("thrippy_id", linkID))
		return err
	}
	return nil
}

// thrippyClient initializes and returns a Thrippy gRPC client connection
// and client stub. The caller is responsible for closing the connection.
func (c *Config) thrippyClient() (*grpc.ClientConn, thrippypb.ThrippyServiceClient, error) {
	creds, err := c.secureCreds()
	if err != nil {
		return nil, nil, err
	}

	conn, err := grpc.NewClient(c.ThrippyGRPCAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, nil, err
	}

	return conn, thrippypb.NewThrippyServiceClient(conn), nil
}

// secureCreds initializes gRPC client credentials using TLS or mTLS,
// based on CLI flags. Errors abort the application with a log message.
func (c *Config) secureCreds() (credentials.TransportCredentials, error) {
	// Both TLS and mTLS.
	caPath := c.thrippyServerCACert
	nameOverride := c.thrippyServerNameOverride
	// Only mTLS.
	certPath := c.thrippyClientCert
	keyPath := c.thrippyClientKey

	// Using mTLS requires the client's X.509 PEM-encoded public cert
	// and private key. If one of them is missing it's an error.
	if certPath == "" && keyPath != "" {
		return nil, errors.New("missing client public cert file for gRPC client with mTLS")
	}
	if certPath != "" && keyPath == "" {
		return nil, errors.New("missing client private key file for gRPC client with mTLS")
	}

	// If both of them are missing, we use TLS.
	if certPath == "" && keyPath == "" {
		return newClientTLSFromFile(caPath, nameOverride, nil)
	}

	// If all 3 are specified, we use mTLS.
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client PEM key pair for gRPC client with mTLS: %w", err)
	}

	return newClientTLSFromFile(caPath, nameOverride, []tls.Certificate{cert})
}

// newClientTLSFromFile constructs TLS credentials from the provided root
// certificate authority certificate file(s) to validate server connections.
//
// This function is based on [credentials.NewClientTLSFromFile], but uses
// TLS 1.3 as the minimum version (instead of 1.2), and support mTLS too.
func newClientTLSFromFile(caPath, serverNameOverride string, certs []tls.Certificate) (credentials.TransportCredentials, error) {
	b, err := os.ReadFile(caPath) //gosec:disable G304 // Specified by admin by design.
	if err != nil {
		return nil, fmt.Errorf("failed to read server CA cert file for gRPC client: %w", err)
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(b) {
		return nil, errors.New("failed to parse server CA cert file for gRPC client")
	}

	cfg := &tls.Config{
		RootCAs:    cp,
		ServerName: serverNameOverride,
		MinVersion: tls.VersionTLS13,
	}
	if len(certs) > 0 {
		cfg.Certificates = certs
	}

	return credentials.NewTLS(cfg), nil
}

func (c *Config) thrippyLinkID(ctx workflow.Context, userID, channelID string) (string, error) {
	if len(userID) > 0 && userID[0] == 'B' {
		return "", nil // Slack bot, not a real user.
	}

	user, optedIn, err := data.SelectUserBySlackID(ctx, userID)
	if err != nil {
		return "", activities.AlertError(ctx, c.AlertsChannel, "", err, "User ID", userID)
	}

	if !optedIn {
		msg := ":warning: Cannot mirror this in the PR, you need to run this slash command: `/revchat opt-in`"
		return "", activities.PostEphemeralMessage(ctx, channelID, userID, msg)
	}

	return user.ThrippyLink, nil
}
