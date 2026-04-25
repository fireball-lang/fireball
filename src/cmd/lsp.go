package main

import (
	"context"
	"fireball/core"
	"fireball/lsp"
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"
)
import "github.com/owenrumney/go-lsp/server"

func getLspCmd() *cobra.Command {
	core.EndProfiler()

	var port uint16

	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Starts a language server",
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := &lsp.Handler{}
			srv := server.NewServer(handler)

			var transport io.ReadWriteCloser

			if port == 0 {
				transport = server.RunStdio()
			} else {
				listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
				if err != nil {
					return err
				}

				//goland:noinspection ALL
				defer listener.Close()

				conn, err := listener.Accept()
				if err != nil {
					return err
				}

				transport = conn
			}

			return srv.Run(context.Background(), transport)
		},
	}

	cmd.Flags().Uint16VarP(&port, "port", "p", 0, "use a TCP connection with the specified port instead of STDIO")

	return cmd
}
