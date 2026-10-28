# Security Policy

## Reporting a vulnerability

Please **do not open a public issue** for a security problem.

Report it privately through GitHub:
[Report a vulnerability](https://github.com/deaprima/linkforge/security/advisories/new).

Include what you found, the steps to reproduce it, and the impact you expect. You will get a first reply within a few
days. I will keep you updated while it is fixed, and credit you in the release notes if you want that.

## Scope

In scope: the Gateway, Auth, Link and Analytics services, the web app, and the Docker setup in this repository.

Particularly interesting to me:

- authentication, session and token handling (cookies, refresh token rotation, API keys, password reset)
- the password protection on short links and its brute-force limits
- open redirects, SSRF and injection through link destinations
- anything that lets one account read or change another account's links or analytics

Out of scope: findings that need a compromised machine or browser, missing security headers on a local development
setup, and denial of service through sheer volume.

## Supported versions

Only the latest commit on `main` is supported. LinkForge has no tagged releases yet.

## Running your own instance

- Set a long random `JWT_SECRET` and keep `COOKIE_SECURE=true`.
- Behind a reverse proxy, set `TRUSTED_PROXIES`, otherwise every visitor shares one rate limit.
- Configure `SMTP_*` so password reset links are emailed instead of written to the Auth service log.
- Never commit your `.env` file.
