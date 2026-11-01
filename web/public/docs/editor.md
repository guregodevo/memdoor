# Memdoor in your editor

The same coder in VS Code, Zed or JetBrains. The editor's chat panel sends
the prompt. The coder works in the open folder, on your key or ChatGPT
plan. Nothing else to install: the editor starts `memdoor acp`, and Memdoor
speaks the [Agent Client Protocol](https://agentclientprotocol.com) the
editors host.

The chat shows what the window shows: the streaming answer, the files
read, the edits placed at the file, the commands run, the green or red of
the build and tests. It prompts for a question, or for an approval under
`MEMDOOR_APPROVE=changes`. Stop ends the turn, as `Esc` does.

## VS Code

1. Install the **ACP Client** extension (id `formulahendry.acp-client`).
2. In `settings.json`:

   ```json
   "acp.agents": { "Memdoor": { "command": "memdoor", "args": ["acp"] } }
   ```

   `memdoor` is on PATH after the installer; otherwise its full path
   (`~/.local/bin/memdoor`).
3. Open a folder, click the ACP Client icon in the Activity Bar, pick
   **Memdoor**, type.

Any other ACP client extension works the same way under its own setting
name.

## Zed

Agent panel → add a custom agent: command `memdoor`, argument `acp`. Zed
sends the open folder as the working directory.

## JetBrains

AI assistant settings → external agents → add an Agent Client Protocol
agent with the command `memdoor acp`.

## Good to know

- Each editor session is a conversation of its own. `memdoor resume` in a
  terminal lists them; a session started in the editor continues in the
  window.
- The first run sets the machine up, as `memdoor tui` does. No key yet: the
  chat says so (`/connect` in the window, or `export OPENROUTER_API_KEY=…`
  before the editor starts).
- `/model`, `/workflow` and the other slash commands are the window's; the
  editor sends plain prompts. A model pinned in the window holds for new
  conversations.
- Where prompts go: [Where your data goes](/docs/security).

## Next

- **[Headless, then attached](/docs/headless)** — leave a run alone and come back into it live.
