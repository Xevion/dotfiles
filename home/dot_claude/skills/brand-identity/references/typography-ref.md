# Typography Reference

## Personality-to-Font Mapping

Map the project's personality to appropriate Google Fonts pairings (heading + body).

### Technical / Developer-Focused
| Heading | Body | Vibe |
|---|---|---|
| JetBrains Mono | Inter | Developer tool, precise |
| IBM Plex Mono | IBM Plex Sans | Enterprise/serious dev tool |
| Fira Code | Source Sans 3 | Open source, Mozilla heritage |
| Space Mono | Space Grotesk | Futuristic, space-age dev |

### Clean / Modern / Minimal
| Heading | Body | Vibe |
|---|---|---|
| Inter | Inter | Swiss, neutral, versatile |
| Plus Jakarta Sans | Inter | Friendly modern |
| Outfit | Source Sans 3 | Contemporary, approachable |
| General Sans | General Sans | Geometric, clean |

### Bold / Confident / Startup
| Heading | Body | Vibe |
|---|---|---|
| Satoshi | Inter | Y Combinator energy |
| Cabinet Grotesk | DM Sans | Bold, startup-y |
| Sora | Nunito Sans | Rounded, friendly-bold |
| Manrope | Manrope | Geometric, distinctive |

### Playful / Creative / Fun
| Heading | Body | Vibe |
|---|---|---|
| Fredoka | Nunito | Rounded, toy-like |
| Baloo 2 | Quicksand | Bubbly, casual |
| Rubik | Rubik | Slightly rounded, game-like |
| Comfortaa | DM Sans | Soft, rounded |

### Serious / Enterprise / Professional
| Heading | Body | Vibe |
|---|---|---|
| DM Serif Display | DM Sans | Editorial, trustworthy |
| Merriweather | Source Sans 3 | Traditional, readable |
| Lora | Open Sans | Elegant, professional |
| Playfair Display | Lato | High-end, editorial |

## Safe System Font Stacks (No External Dependencies)

For projects that prefer zero external font dependencies:

```css
/* Technical */
font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, 'DejaVu Sans Mono', monospace;

/* Clean modern */
font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;

/* Serif/editorial */
font-family: 'Iowan Old Style', 'Palatino Linotype', 'URW Palladio L', P052, serif;
```

## Font Pairing Principles

- **Contrast heading and body**: Don't use the same font for both unless it's a superfamily (like IBM Plex or DM Sans/Serif)
- **Match x-height**: Fonts with similar x-heights look more harmonious together
- **Limit to 2 fonts**: heading + body. A third font (monospace for code) is acceptable in dev contexts.
- **Weight hierarchy**: Headings at 600-800 weight, body at 400. Use weight, not different fonts, for visual hierarchy within body text.
