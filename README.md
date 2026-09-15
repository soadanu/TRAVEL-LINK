# ASAA Travel Ticket Application System

Features customer authentication, ticket applications, location-based price tags, and an Admin Dashboard with full track records of every user application.

## Credentials

- **Customer Login (`/login`):**
  - Email: `traveler@example.com`
  - Password: `user123`
  - *(or register a new account on `/register`)*

- **Admin Login (`/admin-login`):**
  - Username: `admin`
  - Password: `adminpassword123`

## Features Built
1. **Customer Login & Register:** Customers log in before filling out their ticket application.
2. **Location Fare Tags:** Dynamic destination fare lookup and admin custom price quoting.
3. **Admin Track Record Dashboard:** Displays live application records of every customer applying for a ticket (Name, Email, Phone, Destination, Mode, Travel Date/Time, Status, Seat Number).

## How to Run

```bash
go run main.go
```
Visit `http://localhost:8080`.
