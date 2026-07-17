package main

import (
	"context"
	"fireball/cfg"
	"fireball/core"
	"fireball/lsp"
	"log/slog"
	"os"
	"strconv"

	"github.com/fireball-lang/protocol"
	"go.lsp.dev/jsonrpc2"

	"net"

	"github.com/spf13/cobra"
)

func getLspCmd() *cobra.Command {
	port := uint16(0)
	targetOs := TargetOsValue{Value: cfg.GetHost().TargetOs}

	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Starts a language server",
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			core.EndProfiler()

			stream, logger, err := getStream(port)
			if err != nil {
				return err
			}

			server := &lsp.Server{}
			_, conn, client := protocol.NewServer(context.Background(), server, stream, logger)

			server.Logger = logger
			server.Client = client

			server.Env = cfg.GetHost()
			server.Env.TargetOs = targetOs.Value

			logger.Info("Connected")
			<-conn.Done()

			return nil
		}),
	}

	cmd.Flags().Uint16VarP(&port, "port", "p", 0, "use a TCP connection with the specified port instead of STDIO")
	cmd.Flags().VarP(&targetOs, "target", "t", "override the target OS")

	return cmd
}

type stdioRWC struct{}

func (s stdioRWC) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (s stdioRWC) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (s stdioRWC) Close() error                { return nil }

func getStream(port uint16) (jsonrpc2.Stream, *slog.Logger, error) {
	// STDIO
	if port == 0 {
		return jsonrpc2.NewStream(stdioRWC{}), slog.New(slog.DiscardHandler), nil
	}

	// TCP
	logger := slog.Default()
	logger.Info("Listening on :" + strconv.Itoa(int(port)))

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return nil, nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer listener.Close()

	conn, err := listener.Accept()
	if err != nil {
		return nil, nil, err
	}

	return jsonrpc2.NewStream(conn), logger, nil
}
