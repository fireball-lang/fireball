package lsp

import (
	"context"
	"github.com/MineGame159/protocol"
	"github.com/spf13/cobra"
	"go.lsp.dev/jsonrpc2"
	"go.uber.org/zap"
	"net"
	"os"
	"strconv"
)

func Command() *cobra.Command {
	port := uint16(0)

	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Language server for Fireball",
		RunE: func(_ *cobra.Command, args []string) error {
			stream, logger, err := getStream(port)
			if err != nil {
				return err
			}

			server := newServer()

			_, conn, client := protocol.NewServer(context.Background(), server, stream, logger)

			server.logger = logger
			server.client = client

			logger.Info("Connected")
			<-conn.Done()

			return nil
		},
	}

	cmd.Flags().Uint16VarP(&port, "port", "p", 0, "Port to start the LSP server on. If not specified the LSP server will use STDOUT / STDIN.")

	return cmd
}

func getStream(port uint16) (jsonrpc2.Stream, *zap.Logger, error) {
	// STDIO
	if port == 0 {
		return jsonrpc2.NewStream(os.Stdout), zap.NewNop(), nil
	}

	// TCP
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, nil, err
	}

	logger.Info("Listening on :" + strconv.Itoa(int(port)))

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return nil, nil, err
	}

	conn, err := listener.Accept()
	if err != nil {
		return nil, nil, err
	}

	return jsonrpc2.NewStream(conn), logger, nil
}
