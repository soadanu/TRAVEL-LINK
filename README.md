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

## Running it

```bash
go mod tidy
go run .
```

Then visit `http://localhost:8080`.

- Customer demo login: `traveler@example.com` / `user123`
- Admin login (same form): `admin` / `adminpassword123`

## Still worth fixing (not part of this request, flagged for later)

- Admin/customer "sessions" are just a flag in `localStorage` — there's
  no real server-side session or token, so this isn't secure for
  production.
- All data (users, applications, pricing) is in-memory and resets on
  restart.
- SMTP and Flutterwave credentials in `main.go` / `book.html` are
  placeholders — swap in real ones (ideally via environment variables,
  not hardcoded) before going live.
