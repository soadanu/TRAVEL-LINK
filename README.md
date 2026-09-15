# ASAA Travel — Rebuild Notes

## What changed from the previous version

1. **Admin Portal button removed from the top nav.**
   `index.html` no longer links to `/admin-login`. There is a single
   "Login" link for everyone.

2. **One login form, two roles.**
   `/login` now posts to `/api/auth/login`, which:
   - checks the submitted credentials against the admin username/password
     first,
   - and if they don't match, falls back to checking the customer
     accounts map.

   The response includes a `role` field (`"admin"` or `"customer"`).
   The frontend reads that and redirects to `/admin` (setting the admin
   token) or `/` (storing the customer session) accordingly.

   The old `admin_login.html` page and the separate
   `/api/auth/admin-login` endpoint were removed since they're no longer
   needed — everything goes through `/login`.

3. **Admin can now edit an existing price tag at any time.**
   The pricing table on `/admin` has an **Edit** button on every row.
   Clicking it pre-fills the form with that destination, mode, and
   current price, and switches the submit button to "Update Location
   Price Tag." Submitting posts to the same `/api/admin/pricing`
   endpoint, which upserts by the `"Destination - Mode"` key — so the
   same backend call both creates new price tags and overwrites
   existing ones. A "Cancel" link clears the form back to add-mode.

4. **Checkout currency switched from USD to NGN.**
   Flutterwave's Standard Checkout only offers card payments for USD —
   bank transfer, USSD, and mobile money all require a local currency.
   `book.html` now checks out in NGN with
   `payment_options: "card, banktransfer, ussd, account"` (the right
   set for Nigeria — `mobilemoney` was actually meant for
   Ghana/Kenya/Uganda, not Nigeria, so it's been dropped). All price
   labels ("USD $..." → "₦...") were updated to match, and the demo
   pricing rules in `main.go` were bumped from token dollar amounts to
   more realistic Naira fares — update those to your real prices
   whenever you're ready.

5. **New "My Tickets" page for customers.**
   Logged-in customers now see a "My Tickets" link in the nav (`/my-tickets`)
   showing every application they've submitted — destination, date,
   status, price, and seat number once assigned — newest first. Backed
   by a new `GET /api/my-applications?email=...` endpoint that filters
   the in-memory applications list by the logged-in customer's email.

6. **Secrets moved to environment variables.**
   `main.go` now reads the admin credentials, SMTP settings, and the
   Flutterwave public key from env vars (with the old values kept as
   local-dev fallbacks so `go run .` still works untouched). It also
   binds to `$PORT` instead of a hardcoded `8080`, which Render (and
   most hosts) require.

## Running it locally

```bash
go mod tidy
go run .
```

Then visit `http://localhost:8080`.

- Customer demo login: `traveler@example.com` / `user123`
- Admin login (same form): `admin` / `adminpassword123`

## Environment variables (for Render / production)

On Render: **Environment** tab → **Add variable** for each of these →
**Save, rebuild, and deploy**. Locally, if you don't set them, the app
falls back to the old demo values automatically.

| Key | Purpose | Example |
|---|---|---|
| `ADMIN_USERNAME` | Admin login identifier | `admin` |
| `ADMIN_PASSWORD` | Admin login password | a strong password, not the old default |
| `SMTP_EMAIL` | "From" address for ticket emails | `noreply@yourdomain.com` |
| `SMTP_APP_PASSWORD` | App password for that mailbox (not your normal password) | Gmail app password |
| `SMTP_HOST` | SMTP server | `smtp.gmail.com` |
| `SMTP_PORT` | SMTP port | `587` |
| `FLUTTERWAVE_PUBLIC_KEY` | Your Flutterwave **publishable** key | `FLWPUBK-...` |

Render also injects `PORT` automatically — you don't set that one
yourself, the app just needs to read it, which it now does.

⚠️ Only ever put your Flutterwave **secret** key server-side if you add
real payment verification later — never in an env var that gets read
by client-facing code, and never in a `FLWPUBK`-prefixed variable
(that prefix is the publishable one, safe for the browser).

## Still worth fixing (not part of this request, flagged for later)

- Admin/customer "sessions" are just a flag in `localStorage` — there's
  no real server-side session or token, so this isn't secure for
  production.
- All data (users, applications, pricing) is in-memory and resets on
  restart.
- SMTP and Flutterwave credentials in `main.go` / `book.html` are
  placeholders — swap in real ones (ideally via environment variables,
  not hardcoded) before going live.
