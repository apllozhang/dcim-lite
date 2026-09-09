# dcim-lite frontends

Two UI packages share the same Go API.

| Directory | Purpose | Public Git? |
| --- | --- | --- |
| `ale/` | Internal skin: compiled legacy Vue bundle + ALE branding + captcha/table patches | **No** (trademark + non-redistributable build artifacts) |
| `clean/` | Open-source UI: vanilla JS, login+captcha, tree, rack/device tables | Yes |

## clean/

```bash
# same-origin proxy (nginx) or set window.DCIM_API_BASE
# serve statically, e.g.
python3 -m http.server 5173
# or docker nginx with /api → backend:8080
```

Features: login with 4-digit captcha, dashboard counts, resource tree, rack list (search/status/sort/page 10/20/30), device list (search/lifecycle/sort/page).

## ale/

Drop-in replacement for internal deployments (e.g. 10.20.30.203:19173).  
Do not push this folder to a public repository.
