use zed_extension_api as zed;

struct FireballExt {}

impl zed::Extension for FireballExt {
    fn new() -> Self
    where
        Self: Sized,
    {
        FireballExt {}
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<zed::Command> {
        let (platform, _arch) = zed::current_platform();

        let environment = match platform {
            zed::Os::Mac | zed::Os::Linux => worktree.shell_env(),
            zed::Os::Windows => vec![],
        };

        if let Ok(lsp_settings) = zed::settings::LspSettings::for_worktree("fireball", worktree) {
            if let Some(binary) = lsp_settings.binary {
                if let Some(path) = binary.path {
                    return Ok(zed::Command {
                        command: path,
                        args: binary.arguments.unwrap_or_default(),
                        env: environment,
                    });
                }
            }
        }

        if let Some(path) = worktree.which("fireball") {
            return Ok(zed::Command {
                command: path,
                args: vec!["lsp".to_string()],
                env: vec![],
            });
        }

        Err("failed to find 'fireball' in $PATH".to_string())
    }

    fn language_server_initialization_options(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<Option<zed::serde_json::Value>> {
        let options = zed::settings::LspSettings::for_worktree("fireball", worktree)
            .ok()
            .and_then(|lsp_settings| lsp_settings.initialization_options.clone())
            .unwrap_or_default();

        Ok(Some(options))
    }

    fn language_server_workspace_configuration(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<Option<zed::serde_json::Value>> {
        let settings = zed::settings::LspSettings::for_worktree("fireball", worktree)
            .ok()
            .and_then(|lsp_settings| lsp_settings.settings.clone())
            .unwrap_or_default();

        Ok(Some(settings))
    }
}

zed::register_extension!(FireballExt);
