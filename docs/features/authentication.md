# Authentication

Two identities, one command.

- **memdoor.ai** is the account that holds the Pro seat: remote control
  through the memdoor.ai relay, and every workflow run's state kept on memdoor.ai
  (runs with the laptop closed are coming). A seat never
  supplies a model key: inference is always on the person's own. Workflows need no account. `memdoor login you@example.com` emails a six-digit code;
  typing it is the sign-in. The token is kept in `~/.memdoor/billing-token` (0600).
- **This Mac** is the user your own gateway knows you as. The window and every
  CLI command talk to the gateway as that user. `memdoor login` sets it up
  (a generated password in `~/.memdoor/local-login`, the session in
  `~/.memdoor/credentials.json`) and renews it when it lapses. Without a
  memdoor.ai account, `memdoor setup` creates it.

```bash
memdoor login you@example.com     # sign in; the engine session is set up behind it
memdoor account status            # both identities, one line each
memdoor logout                    # sign out of memdoor.ai
```

`memdoor logout` forgets the seat and leaves this Mac's session alone: the
agent on your own key keeps working, and nothing would renew that session
without a memdoor.ai account.

## By hand (scripts, test steps)

`memdoor auth` is the engine's sign-in without the account, hidden from help:

```bash
memdoor auth login-direct --email you@work.com --password '…'   # 30-day session
memdoor auth token                                               # the token, for curl
memdoor auth whoami                                              # the engine user and role
```

The browser flow `memdoor auth login` was removed on 2026-10-03: it pointed
the gateway at a local callback port nothing listened on, so the token never
arrived.
