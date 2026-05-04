# Multi-User Quickstart

This quickstart is for a LAN or VPN deployment where a reverse proxy
authenticates writers and forwards the authenticated user in
`X-Forwarded-User`.

## Start The App

```sh
export MULTI_USER=1
export WORKFLOW_STORE_DRIVER=sqlite
docker compose up -d --build app
curl -i http://127.0.0.1:8080/api/workflow/artifacts
```

The final curl should return `401` because direct unauthenticated requests are
rejected in multi-user mode.

## Trusted Header Contract

Only the reverse proxy may send `X-Forwarded-User` to the app. Do not expose
the app container port directly outside the host when `MULTI_USER=1`.

Required proxy behavior:

- Authenticate every request before proxying to Note Maker.
- Strip any incoming `X-Forwarded-User` from clients.
- Set `X-Forwarded-User` to the authenticated username or email.
- Restrict direct app access to localhost, the Docker network, or the proxy.

## Caddy Example

Use this behind an authentication plugin or an upstream identity gateway that
sets `{http.auth.user.id}`.

```caddyfile
note-maker.example.internal {
	@directHeader header X-Forwarded-User *
	header @directHeader -X-Forwarded-User

	reverse_proxy 127.0.0.1:8080 {
		header_up X-Forwarded-User {http.auth.user.id}
	}
}
```

If your identity gateway sends the user in a different header, map that header
to `X-Forwarded-User` inside the trusted proxy and strip it from external
requests.

## Nginx Example

```nginx
server {
    listen 443 ssl;
    server_name note-maker.example.internal;

    location / {
        proxy_set_header X-Forwarded-User "";
        proxy_set_header X-Forwarded-User $remote_user;
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:8080;
    }
}
```

## Smoke Test

```sh
curl -i http://127.0.0.1:8080/api/workflow/artifacts
curl -fsS -H 'X-Forwarded-User: alice' \
  http://127.0.0.1:8080/api/workflow/artifacts
curl -fsS -H 'X-Forwarded-User: bob' \
  http://127.0.0.1:8080/api/workflow/artifacts
```

Expected result:

- The first request returns `401`.
- The Alice and Bob requests return `200`.
- Alice and Bob only see rows owned by their own `user_id`.

The automated counterpart is:

```sh
python3 -m pytest tests/e2e/test_multi_user_isolation.py -q
```
