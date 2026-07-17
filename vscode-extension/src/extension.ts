import * as vscode from "vscode";
import * as net from "net";
import { ChildProcess, spawn } from "child_process";
import which from "which";

import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    StreamInfo,
    TransportKind
} from "vscode-languageclient/node";

// for some reason STDIO transport is not working in VSCode but it works in Zed
const USE_TCP = true;
const SPAWN_TCP = true;

let process: ChildProcess | undefined;
let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext) {
    startServer();

    context.subscriptions.push(vscode.workspace.onDidChangeConfiguration(async (e) => {
        if (e.affectsConfiguration("fireball.targetOs")) {
            await stopServer();
            await startServer();
        }
    }));
}

export async function deactivate() {
    await stopServer();
}

async function startServer() {
    if (client !== undefined) {
        return;
    }

    let path = await which("fireball", { nothrow: true });

    if (path === null) {
        vscode.window.showErrorMessage("Failed to find the 'fireball' binary in your PATH");
        return;
    }

    let serverOptions: ServerOptions;

    if (USE_TCP) {
        if (SPAWN_TCP) {
            process = spawn(path, [ "lsp", "-p=9257" ]);
        }

        serverOptions = () => {
            let socket = net.createConnection({
                port: 9257,
            });

            let result: StreamInfo = {
                writer: socket,
                reader: socket
            };

            return Promise.resolve(result);
        };
    } else {
        serverOptions = {
            transport: TransportKind.stdio,
            command: path,
            args: ["lsp"]
        };
    }

    let clientOptions: LanguageClientOptions = {
        documentSelector: [
            {
                scheme: "file",
                language: "fireball"
            }
        ],
        initializationOptions: {
            full_semantic_tokens: true,
            target_os: vscode.workspace.getConfiguration("fireball").get("targetOs")
        }
    };

    client = new LanguageClient(
        "fireball",
        "Fireball",
        serverOptions,
        clientOptions
    );

    console.log("Starting LSP");
    await client.start();
}

async function stopServer() {
    if (client === undefined) {
        return;
    }

    console.log("Stopping LSP");

    await client?.dispose();
    client = undefined;

    process?.kill();
    process = undefined;
}
