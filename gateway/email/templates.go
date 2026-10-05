package email

// emailVerificationTemplate is the HTML template for email verification emails
const emailVerificationTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verify your email - Memdoor</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f3f4f6;">
    <table width="100%" cellpadding="0" cellspacing="0" style="background-color: #f3f4f6; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table width="600" cellpadding="0" cellspacing="0" style="background-color: #ffffff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 40px 40px 20px; text-align: center;">
                            <img src="{{.BaseURL}}/memdoor-logo.png" alt="Memdoor" style="height: 36px;" />
                        </td>
                    </tr>

                    <!-- Body -->
                    <tr>
                        <td style="padding: 20px 40px 40px;">
                            <h2 style="margin: 0 0 20px; color: #1f2937; font-size: 24px;">Welcome, {{.Name}}!</h2>

                            <p style="margin: 0 0 20px; color: #4b5563; font-size: 16px; line-height: 24px;">
                                Thank you for signing up for Memdoor! Please verify your email address by clicking the button below:
                            </p>

                            <table width="100%" cellpadding="0" cellspacing="0" style="margin: 30px 0;">
                                <tr>
                                    <td align="center">
                                        <a href="{{.VerificationURL}}" style="display: inline-block; padding: 14px 40px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: #ffffff; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 16px;">
                                            Verify Email Address
                                        </a>
                                    </td>
                                </tr>
                            </table>

                            <p style="margin: 20px 0 0; color: #6b7280; font-size: 14px; line-height: 20px;">
                                Or copy and paste this link into your browser:
                            </p>
                            <p style="margin: 10px 0; color: #667eea; font-size: 14px; word-break: break-all;">
                                {{.VerificationURL}}
                            </p>

                            <hr style="border: none; border-top: 1px solid #e5e7eb; margin: 30px 0;">

                            <p style="margin: 0; color: #9ca3af; font-size: 14px; line-height: 20px;">
                                <strong>This link expires in 24 hours.</strong>
                            </p>

                            <p style="margin: 10px 0 0; color: #9ca3af; font-size: 14px; line-height: 20px;">
                                If you didn't create this account, please ignore this email.
                            </p>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="padding: 20px 40px; background-color: #f9fafb; border-top: 1px solid #e5e7eb; border-radius: 0 0 8px 8px;">
                            <p style="margin: 0; color: #6b7280; font-size: 12px; text-align: center;">
                                © 2026 Memdoor
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

// passwordResetTemplate is the HTML template for password reset emails
const passwordResetTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset your password - Memdoor</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f3f4f6;">
    <table width="100%" cellpadding="0" cellspacing="0" style="background-color: #f3f4f6; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table width="600" cellpadding="0" cellspacing="0" style="background-color: #ffffff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 40px 40px 20px; text-align: center;">
                            <img src="{{.BaseURL}}/memdoor-logo.png" alt="Memdoor" style="height: 36px;" />
                        </td>
                    </tr>

                    <!-- Body -->
                    <tr>
                        <td style="padding: 20px 40px 40px;">
                            <h2 style="margin: 0 0 20px; color: #1f2937; font-size: 24px;">Reset Your Password</h2>

                            <p style="margin: 0 0 20px; color: #4b5563; font-size: 16px; line-height: 24px;">
                                Hi {{.Name}},
                            </p>

                            <p style="margin: 0 0 20px; color: #4b5563; font-size: 16px; line-height: 24px;">
                                We received a request to reset your password. Click the button below to create a new password:
                            </p>

                            <table width="100%" cellpadding="0" cellspacing="0" style="margin: 30px 0;">
                                <tr>
                                    <td align="center">
                                        <a href="{{.ResetURL}}" style="display: inline-block; padding: 14px 40px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: #ffffff; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 16px;">
                                            Reset Password
                                        </a>
                                    </td>
                                </tr>
                            </table>

                            <p style="margin: 20px 0 0; color: #6b7280; font-size: 14px; line-height: 20px;">
                                Or copy and paste this link into your browser:
                            </p>
                            <p style="margin: 10px 0; color: #667eea; font-size: 14px; word-break: break-all;">
                                {{.ResetURL}}
                            </p>

                            <hr style="border: none; border-top: 1px solid #e5e7eb; margin: 30px 0;">

                            <p style="margin: 0; color: #9ca3af; font-size: 14px; line-height: 20px;">
                                <strong>This link expires in 1 hour.</strong>
                            </p>

                            <p style="margin: 10px 0 0; color: #9ca3af; font-size: 14px; line-height: 20px;">
                                If you didn't request this password reset, please ignore this email or contact support if you have concerns.
                            </p>

                            <p style="margin: 10px 0 0; color: #9ca3af; font-size: 14px; line-height: 20px;">
                                For security, all active sessions will be logged out once you reset your password.
                            </p>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="padding: 20px 40px; background-color: #f9fafb; border-top: 1px solid #e5e7eb; border-radius: 0 0 8px 8px;">
                            <p style="margin: 0; color: #6b7280; font-size: 12px; text-align: center;">
                                © 2026 Memdoor
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

// demoRequestTemplate is the HTML template for demo request notification emails
const demoRequestTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>New Demo Request - Memdoor</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #0a0a12;">
    <table width="100%" cellpadding="0" cellspacing="0" style="background-color: #0a0a12; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table width="600" cellpadding="0" cellspacing="0" style="background-color: #12121e; border-radius: 12px; border: 1px solid rgba(168,85,247,0.2); box-shadow: 0 0 40px rgba(168,85,247,0.1);">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 40px 40px 20px; text-align: center; border-bottom: 1px solid rgba(168,85,247,0.15);">
                            <h1 style="margin: 0; font-size: 28px; background: linear-gradient(135deg, #06b6d4, #8b5cf6, #ec4899); -webkit-background-clip: text; -webkit-text-fill-color: transparent;">Memdoor</h1>
                            <p style="margin: 8px 0 0; color: #9ca3af; font-size: 13px; font-family: monospace;">// new demo request</p>
                        </td>
                    </tr>

                    <!-- Body -->
                    <tr>
                        <td style="padding: 30px 40px 40px;">
                            <h2 style="margin: 0 0 24px; color: #ffffff; font-size: 22px;">Someone wants a demo!</h2>

                            <!-- Email -->
                            <div style="margin: 0 0 16px; padding: 16px 20px; background-color: rgba(6,182,212,0.08); border: 1px solid rgba(6,182,212,0.2); border-radius: 8px;">
                                <p style="margin: 0 0 4px; color: #9ca3af; font-size: 11px; font-family: monospace; text-transform: uppercase; letter-spacing: 1px;">Email</p>
                                <p style="margin: 0; color: #22d3ee; font-size: 16px; font-weight: 600;">{{.Email}}</p>
                            </div>

                            <!-- YouTube Channel -->
                            <div style="margin: 0 0 16px; padding: 16px 20px; background-color: rgba(168,85,247,0.08); border: 1px solid rgba(168,85,247,0.2); border-radius: 8px;">
                                <p style="margin: 0 0 4px; color: #9ca3af; font-size: 11px; font-family: monospace; text-transform: uppercase; letter-spacing: 1px;">YouTube Channel</p>
                                <p style="margin: 0; color: #a78bfa; font-size: 16px; font-weight: 600;">{{.YouTubeChannel}}</p>
                            </div>

                            <!-- Timestamp -->
                            <div style="margin: 0 0 0; padding: 16px 20px; background-color: rgba(236,72,153,0.08); border: 1px solid rgba(236,72,153,0.2); border-radius: 8px;">
                                <p style="margin: 0 0 4px; color: #9ca3af; font-size: 11px; font-family: monospace; text-transform: uppercase; letter-spacing: 1px;">Submitted</p>
                                <p style="margin: 0; color: #f472b6; font-size: 14px;">{{.Time}}</p>
                            </div>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="padding: 20px 40px; border-top: 1px solid rgba(168,85,247,0.15); border-radius: 0 0 12px 12px;">
                            <p style="margin: 0; color: #4b5563; font-size: 12px; text-align: center; font-family: monospace;">
                                Memdoor // Creator AI Agents
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

// inviteTemplate is the HTML template for workspace invite emails
const inviteTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>You're invited to Memdoor</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f3f4f6;">
    <table width="100%" cellpadding="0" cellspacing="0" style="background-color: #f3f4f6; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table width="600" cellpadding="0" cellspacing="0" style="background-color: #ffffff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 40px 40px 20px; text-align: center; background: linear-gradient(135deg, #0a0a12 0%, #1a0f2e 100%); border-radius: 8px 8px 0 0;">
                            <h1 style="margin: 0; color: #a78bfa; font-size: 28px;">Memdoor</h1>
                            <p style="margin: 8px 0 0; color: #9ca3af; font-size: 14px;">Your AI Agent Workspace</p>
                        </td>
                    </tr>

                    <!-- Body -->
                    <tr>
                        <td style="padding: 30px 40px 40px;">
                            <h2 style="margin: 0 0 20px; color: #1f2937; font-size: 24px;">You're Invited!</h2>

                            <p style="margin: 0 0 20px; color: #4b5563; font-size: 16px; line-height: 24px;">
                                <strong>{{.InviterName}}</strong> has invited you to join their workspace on Memdoor.
                            </p>

                            {{if .ChannelName}}
                            <div style="margin: 0 0 20px; padding: 16px; background-color: #f0f9ff; border-left: 4px solid #667eea; border-radius: 4px;">
                                <p style="margin: 0; color: #4b5563; font-size: 14px;">
                                    You've been invited to the <strong>#{{.ChannelName}}</strong> channel.
                                </p>
                            </div>
                            {{end}}

                            {{if .Message}}
                            <div style="margin: 0 0 20px; padding: 16px; background-color: #faf5ff; border-left: 4px solid #a78bfa; border-radius: 4px;">
                                <p style="margin: 0; color: #6b7280; font-size: 13px; font-style: italic;">
                                    "{{.Message}}"
                                </p>
                                <p style="margin: 8px 0 0; color: #9ca3af; font-size: 12px;">
                                    — {{.InviterName}}
                                </p>
                            </div>
                            {{end}}

                            <p style="margin: 0 0 20px; color: #4b5563; font-size: 16px; line-height: 24px;">
                                Memdoor is a platform where AI agents and humans collaborate together. Join and start working with intelligent agents that help you get things done.
                            </p>

                            <table width="100%" cellpadding="0" cellspacing="0" style="margin: 30px 0;">
                                <tr>
                                    <td align="center">
                                        <a href="{{.InviteURL}}" style="display: inline-block; padding: 14px 40px; background: linear-gradient(135deg, #06b6d4 0%, #8b5cf6 50%, #ec4899 100%); color: #ffffff; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 16px;">
                                            Join Workspace
                                        </a>
                                    </td>
                                </tr>
                            </table>

                            <p style="margin: 20px 0 0; color: #6b7280; font-size: 14px; line-height: 20px;">
                                Or copy and paste this link into your browser:
                            </p>
                            <p style="margin: 10px 0; color: #667eea; font-size: 14px; word-break: break-all;">
                                {{.InviteURL}}
                            </p>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="padding: 20px 40px; background-color: #f9fafb; border-top: 1px solid #e5e7eb; border-radius: 0 0 8px 8px;">
                            <p style="margin: 0; color: #6b7280; font-size: 12px; text-align: center;">
                                © 2026 Memdoor
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`
