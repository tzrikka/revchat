package slack

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/tzrikka/revchat/internal/log"
	thrippypb "github.com/tzrikka/thrippy-api/thrippy/v1"
)

const (
	grpcTimeout = 3 * time.Second

	createLinkTimeout = 15 * time.Second

	pollLinkTimeout    = 2 * time.Second
	pollLinkInterval   = 1 * time.Second
	waitForLinkTimeout = 1 * time.Minute

	ErrLinkAuthzTimeout = "link not authorized yet" // For some reason errors.Is() doesn't work across Temporal.
)

type linkData struct {
	ID    string `json:"id"`
	Nonce string `json:"nonce"`
}

// createThrippyLink executes [createThrippyLinkActivity] as a local activity.
func (c Config) createThrippyLink(ctx workflow.Context) (string, string, error) {
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
func (c Config) createThrippyLinkActivity(ctx context.Context) (*linkData, error) {
	// Initialize a Thrippy client.
	creds, err := c.secureCreds()
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(c.thrippyGRPCAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := thrippypb.NewThrippyServiceClient(conn)
	ctx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	// Create the link to generate an ID.
	createReq := thrippypb.CreateLinkRequest_builder{
		Template: proto.String(c.thrippyLinksTemplate),
		OauthConfig: thrippypb.OAuthConfig_builder{
			ClientId:     proto.String(c.thrippyLinksClientID),
			ClientSecret: proto.String(c.thrippyLinksClientSecret),
		}.Build(),
	}.Build()

	createResp, err := client.CreateLink(ctx, createReq)
	if err != nil {
		return nil, err
	}

	// Retrieve the new link's configuration to get its nonce.
	getReq := thrippypb.GetLinkRequest_builder{LinkId: proto.String(createResp.GetLinkId())}.Build()
	getResp, err := client.GetLink(ctx, getReq)
	if err != nil {
		return nil, err
	}

	return &linkData{ID: createResp.GetLinkId(), Nonce: getResp.GetOauthConfig().GetNonce()}, nil
}

// waitForThrippyLinkCreds waits for the user to complete
// the OAuth flow for the Thrippy link with the given ID.
func (c Config) waitForThrippyLinkCreds(ctx workflow.Context, linkID string) error {
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: waitForLinkTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    pollLinkInterval,
			BackoffCoefficient: 1.0,
		},
	})

	return workflow.ExecuteLocalActivity(ctx, c.pollThrippyLinkActivity, linkID).Get(ctx, nil)
}

func (c Config) pollThrippyLinkActivity(ctx context.Context, linkID string) error {
	creds, err := c.secureCreds()
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(c.thrippyGRPCAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, pollLinkTimeout)
	defer cancel()

	req := thrippypb.GetCredentialsRequest_builder{LinkId: proto.String(linkID)}.Build()
	resp, err := thrippypb.NewThrippyServiceClient(conn).GetCredentials(ctx, req)
	if err != nil {
		return err
	}

	if len(resp.GetCredentials()) == 0 {
		return errors.New(ErrLinkAuthzTimeout)
	}

	return nil
}

// deleteThrippyLink deletes the Thrippy link with the given ID. This is a
// best-effort cleanup in case the user opted-in but didn't authorize us in time.
func (c Config) deleteThrippyLink(ctx workflow.Context, linkID string) error {
	creds, err := c.secureCreds()
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(c.thrippyGRPCAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := thrippypb.NewThrippyServiceClient(conn)
	grpcCtx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
	defer cancel()

	req := thrippypb.DeleteLinkRequest_builder{LinkId: proto.String(linkID)}.Build()
	if _, err = client.DeleteLink(grpcCtx, req); err != nil {
		log.Error(ctx, "failed to delete Thrippy link", "error", err, "link_id", linkID)
		return err
	}
	return nil
}

// secureCreds initializes gRPC client credentials using TLS or mTLS,
// based on CLI flags. Errors abort the application with a log message.
func (c Config) secureCreds() (credentials.TransportCredentials, error) {
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
		return nil, fmt.Errorf("failed to load client PEM key pair for gRPC client with mTLS: %v", err)
	}

	return newClientTLSFromFile(caPath, nameOverride, []tls.Certificate{cert})
}

// newClientTLSFromFile constructs TLS credentials from the provided root
// certificate authority certificate file(s) to validate server connections.
//
// This function is based on [credentials.NewClientTLSFromFile], but uses
// TLS 1.3 as the minimum version (instead of 1.2), and support mTLS too.
func newClientTLSFromFile(caPath, serverNameOverride string, certs []tls.Certificate) (credentials.TransportCredentials, error) {
	b, err := os.ReadFile(caPath) //gosec:disable G304 -- specified by admin by design
	if err != nil {
		return nil, fmt.Errorf("failed to read server CA cert file for gRPC client: %v", err)
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
