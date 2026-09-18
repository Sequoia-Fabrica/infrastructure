# Sequoia Fabrica Brand & Style Guide (infrastructure)

Reference for keeping the self-hosted services (authentik login, Grafana,
Etherpad, Multipass...) visually consistent with sequoiafabrica.org. Derived
from the landing page (`sequoia-fabrica-landing-page`) and the blog.

---

## Identity

- **Name**: Sequoia Fabrica (two words, title case; never "SeqFab" in UI)
- **Domains**: sequoiafabrica.org (public), sequoia.garden (member services)
- **Voice**: warm, hands-on, a little botanical. Infrastructure is "the garden";
  hosts are `soil` and `nursery`; the login heading is "Welcome back to the garden".

---

## Logo

The mark is a **circuit-board tree**: a rounded canopy of traces ending in vias,
a trunk, and three ground-symbol roots. The wordmark is a hand-cut, slightly
irregular all-caps "SEQUOIA FABRICA".

| Asset                          | Path                                                                  | Use                                  |
|--------------------------------|-----------------------------------------------------------------------|--------------------------------------|
| Tree only (line art, dark ink) | `ansible/roles/sequoia_fabrica/files/authentik/sequoia_fabrica_tree.svg`           | Icons, small marks                   |
| Tree + wordmark, self-theming  | `ansible/roles/sequoia_fabrica/files/authentik/sequoia_fabrica_lockup.svg`         | authentik login card, headers        |
| Favicon 128 px                 | `ansible/roles/sequoia_fabrica/files/authentik/sequoia_fabrica_favicon.png`        | Browser tab (tea green tree on emerald) |
| Flow background                | `ansible/roles/sequoia_fabrica/files/authentik/sequoia_fabrica_flow_background.svg`| authentik flow pages                 |
| Stacked / horizontal / vertical| `sequoia-fabrica-landing-page/public/sf_logo_*.svg`                    | Web (tea green on emerald)           |

Rules:

- The mark is monochrome. On light surfaces use emerald-800 ink; on emerald or
  dark surfaces use tea green. The lockup SVG does this itself via
  `prefers-color-scheme`.
- Keep clear space at least the height of the roots on all sides.
- Do not add gradients, outlines or shadows to the mark itself.

---

## Colour

Tailwind tokens from the landing page's `tailwind.config.ts`.

| Name            | Hex       | Use                                              |
|-----------------|-----------|--------------------------------------------------|
| **Emerald 800** | `#065f46` | Primary. Nav, footer, buttons, light-mode ink    |
| Emerald 900     | `#064e3b` | Deep backgrounds, sidebar                        |
| Emerald 950     | `#022c22` | Darkest background, header bars                  |
| Emerald 500     | `#10b981` | Highlights, glow, active states                  |
| **Tea green**   | `#c3f9c3` | Wordmark and text on emerald, dark-mode ink      |
| Tea green 200   | `#10a210` | Hover on tea-green text                          |
| **Orange 700**  | `#c2410c` | Body links (hover: orange 900 `#7c2d12`)         |
| Orange 400      | `#fb923c` | Links on dark surfaces                           |
| White smoke     | `#f7f4f3` | Page / card background in light mode             |

The blog uses a separate warm "solar" palette (brown `#543018`, tan `#f8e6bf`,
amber `#f8b92b`); keep that to the blog.

---

## Typography

System sans stack, no web fonts:

```css
font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto,
  "Helvetica Neue", Arial, "Noto Sans", sans-serif;
```

Headings bold; body 16 px / 1.6 line-height. Monospace for hostnames, IPs and
code.

---

## Where it is applied

- **authentik** (`terraform/authentik/`): brand, custom CSS, branded login flow.
- **Grafana**: OAuth button label "Sequoia Garden Login" (`tasks/grafana.yml`).
- **Etherpad**: title "Sequoia Fabrica Etherpad", `colibris` skin.
