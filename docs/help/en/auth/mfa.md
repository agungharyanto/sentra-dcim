---
title: Two-step verification (MFA)
audience: user
order: 26
---

# Two-step verification (MFA)

## Purpose

Every Sentra DCIM account must use **two-step verification**: besides the password you enter a 6-digit code from an
authenticator app on your phone. If your password leaks, nobody can sign in without your phone.

Supported apps (free): **Google Authenticator**, **Microsoft Authenticator** or **Authy**. They all use the same
standard (time-based codes, 6 digits, changing every 30 seconds).

## Who can use it

Every user, whatever the role. Without MFA an account cannot use any menu.

## Steps: first setup

This appears automatically at your first sign-in (or after an Admin reset your MFA).

![Set up two-step verification page with three steps and the Start button](/api/v1/help/assets/help-mfa-intro-en.png)

### 1. Install an authenticator app on your phone

Open the Play Store (Android) or App Store (iPhone), search for **Google Authenticator** (or Microsoft
Authenticator / Authy) and install it.

### 2. Add a new account in the app

Open the app and tap the **+** button (Add account / Add a code).

![Authenticator app screen with the add button at the bottom right](/api/v1/help/assets/mfa-phone-add.svg)

### 3. Scan the QR code from Sentra DCIM

In Sentra DCIM select **Start** until the QR code appears. On the phone choose **Scan a QR code** and point the camera
at the QR code on the computer screen.

![Sentra DCIM page with the QR code, account, setup key and the 6-digit code field (sample data)](/api/v1/help/assets/help-mfa-qr-en.png)

![Phone camera framing the QR code to scan](/api/v1/help/assets/mfa-phone-scan.svg)

Cannot scan (e.g. broken camera)? In the app choose **Enter a setup key**, then type the **Account** and **Key**
shown below the QR code. Key type: **Time based**.

### 4. Type the 6-digit code to activate

The app shows a **Sentra DCIM** account with a 6-digit code that changes every 30 seconds. Type that code into
**6-digit code** and select **Activate**.

![The app shows the 6-digit code of the Sentra DCIM account with a countdown](/api/v1/help/assets/mfa-phone-code.svg)

### 5. Save the 10 recovery codes

Once active, Sentra DCIM shows **10 recovery codes** — **only once**. Keep them somewhere safe (a password manager,
or printed/written down in a locked drawer), **not** on the same phone. Tick *I have saved the recovery codes*, then
select **Continue**.

![List of 10 recovery codes with the shown-once warning and the confirmation checkbox (sample codes)](/api/v1/help/assets/help-mfa-recovery-en.png)

## Steps: everyday sign-in

1. Sign in with your username and password as usual.
2. Open the authenticator app and look at the 6-digit code of the **Sentra DCIM** account.
3. Type the code and select **Verify**. A code that was used cannot be used again; if it is refused, wait for the
   next code (at most 30 seconds).

![Two-step verification page with the 6-digit code field and the recovery code link](/api/v1/help/assets/help-mfa-verify-en.png)

## Phone lost, broken or replaced

- **You still have recovery codes:** on the verification page choose *No phone? Use a recovery code* and type one of
  them. Each recovery code works only once. After signing in, open **Account security** and create new recovery codes.
- **No recovery codes:** contact an Admin. An Admin can **reset your MFA**; at your next sign-in you set it up again
  from step 1.
- **New phone:** ask an Admin to reset your MFA, then set it up on the new phone.

## Account security

User menu (top right) → **Account security** shows the MFA status, since when it is active, and how many recovery
codes are left. There you can **create new recovery codes** (needs your current 6-digit code; all old codes stop
working at once).

## Safeguards

| Rule | Details |
|---|---|
| Repeated wrong codes | Counted together with wrong passwords: 5 times → account locked for 15 minutes and all sessions ended |
| One-time codes | A code that was accepted cannot be used again |
| MFA secret | Stored encrypted; nobody can see it again after setup, Admins included |

## Related API endpoints

| Method & path | Details |
|---|---|
| `GET /api/v1/auth/mfa` | MFA status of your own account |
| `POST /api/v1/auth/mfa/enrol` | Start the setup (QR code + key) |
| `POST /api/v1/auth/mfa/enrol/confirm` | Activate with the first code; returns the recovery codes |
| `POST /api/v1/auth/mfa/verify` | Verify at sign-in (code or recovery code) |
| `POST /api/v1/auth/mfa/recovery-codes` | Create new recovery codes |
