# Memdoor in your editor

The same coder, in VS Code, Zed or JetBrains: your editor's chat panel sends
the prompt, and the coder works in the folder you have open, on your own key
or your ChatGPT plan, exactly as it does in the terminal. Nothing else to
install on the Memdoor side: the editor starts `memdoor acp`, and Memdoor
speaks the [Agent Client Protocol](https://agentclientprotocol.com) the
editors host.

What you see in the chat is what the window shows: the coder's answer as it
streams, every file it reads, every edit (placed at the file), every command
it runs, the green or red of the build and the tests, and a prompt whenever it
needs you: a question, or an approval if you run with
`MEMDOOR_APPROVE=changes`. Stop ends the turn like `Esc` does.

## VS Code

1. Install the **ACP Client** extension (search "ACP Client" in the
   Extensions view; its id is `formulahendry.acp-client`).
2. In your settings (`settings.json`):

   ```json
   "acp.agents": { "Memdoor": { "command": "memdoor", "args": ["acp"] } }
   ```

   `memdoor` is on your PATH after the installer; use its full path
   (`~/.local/bin/memdoor`) if the extension cannot find it.
3. Open a folder, click the ACP Client icon in the Activity Bar, pick
   **Memdoor**, and type.

Any other ACP client extension works the same way; the setting's name is the
extension's.

## Zed

Agent panel → add a custom agent: command `memdoor`, argument `acp`. Zed
sends the folder you have open as the working directory.

## JetBrains

AI assistant settings → external agents → add an Agent Client Protocol
agent with the command `memdoor acp`.

## Good to know

- Each editor session is a conversation of its own; `memdoor resume` in a
  terminal lists them, so a session started in the editor can continue in
  the window.
- The first run sets the machine up, as `memdoor tui` does; a machine with no
  key yet is told so in the chat (`/connect` in the window, or
  `export OPEN_ROUTER_API_KEY=…` before the editor starts).
- `/model`, `/workflow` and the other slash commands are the window's; the
  editor sends plain prompts. Pin a model once in the window and it holds for
  new conversations.
- Where prompts go does not change: [Where your data goes](/docs/security).
