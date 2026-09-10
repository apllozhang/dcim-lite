# ALE skin package (internal / review)

This folder is the **ALE-skinned UI** used on the private deployment.

## Contents

| Item | Origin |
| --- | --- |
| `index.html`, `assets/index-*.js`, view chunks | Compiled Vue 3 app (legacy product build) |
| `assets/element-plus-*` | Element Plus bundle |
| `assets/ale-logo*.png`, `ale-theme.css` | ALE brand assets / theme overrides |
| `assets/captcha-login.js` | Login captcha injector (ours) |
| `assets/table-ux.js` | Sort / column resize / dual h-scroll / ops menu (ours) |

## Review guidance

Focus review on **our** scripts and theme:

- `captcha-login.js`
- `table-ux.js`
- `ale-theme.css` (layout, ALE tokens, table UX)

The compiled Vue/Element bundles are **not** original source of this project.

## License / redistribution

- **Not** intended for public redistribution as open source.
- Brand marks belong to Alcatel-Lucent Enterprise.
- Compiled bundles may not be redistributable under this repo’s license.
- For a clean open-source UI see `../clean/`.

## Deploy note

Serve behind nginx with `/api` → backend; see root `deploy/nginx.conf`.
