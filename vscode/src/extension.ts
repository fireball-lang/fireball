import * as vscode from "vscode";
import * as net from "net";
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    StreamInfo,
    TransportKind
} from "vscode-languageclient/node";
import {ChildProcess, exec} from "child_process";

let process: ChildProcess;
let client: LanguageClient;

export function activate(ctx: vscode.ExtensionContext) {
    // Server options
    let serverOptions: ServerOptions = {
        command: "fireball",
        transport: TransportKind.stdio,
        args: ["lsp"]
    };

    if (true) {
        serverOptions = () => {
            let socket = net.createConnection({
                port: 2077,
            });

            let result: StreamInfo = {
                writer: socket,
                reader: socket
            };

            return Promise.resolve(result);
        };
    }

    if (ctx.extensionMode == vscode.ExtensionMode.Production) {
        process = exec("fireball lsp -p=2077");
    }

    // Client options
    let clientOptons: LanguageClientOptions = {
        documentSelector: [{
            scheme: "file",
            language: "fireball"
        }]
    };

    // Client
    client = new LanguageClient("fireball", "Fireball", serverOptions, clientOptons);
    client.start();

    console.log("Starting LSP");
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }

    console.log("Stopping LSP");

    return client.stop().then(() => {
        process.kill();
    });
}
