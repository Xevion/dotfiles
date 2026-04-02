# TLD Presets & Reference

## Built-in Presets (domain-check)

Run `domain-check --list-presets` for the full current list. Key presets:

| Preset | TLDs |
|---|---|
| startup | com, org, io, ai, tech, app, dev, xyz |
| tech | io, ai, app, dev, tech, cloud, software, digital, codes, systems, network, solutions |
| popular | com, net, org, io, ai, app, dev, tech, me, co, xyz |
| creative | design, art, studio, media, photography, film, music, gallery, graphics, ink |
| classic | com, net, org, info, biz |

## Custom Presets for Developer/OSS Projects

### dev-oss (recommended default for software projects)
com, dev, sh, io, app, tools, run, codes, rs, land, software, systems, tech, ai

### minimal (fast check, most likely to be purchased)
com, dev, io, app, sh

### hacker (short, memorable, dev-culture)
sh, io, rs, so, is, im, to, cc, me, co

### premium-new-gtlds (newer TLDs with lower registration costs)
dev, app, run, tools, codes, software, systems, cloud, digital, network

### anime-otaku (for weeb/doujin/manga/anime projects)
moe, ink, art, fan, life, zone, world, quest, lol, wtf

### nsfw-edgy (adult/provocative/shock-value projects)
xxx, adult, moe, ink, lol, wtf, exposed, zone, party, fun

**Important: Always consider niche/thematic TLDs based on the project's domain.** The presets above are starting points — think about what TLDs resonate with the project's culture and audience, not just what's popular in tech.

## TLD Browsing

For a comprehensive, up-to-date list of all purchasable TLDs with pricing:
- **tld-list.com** — Filterable TLD list with pricing comparison across registrars
- **Porkbun pricing**: `curl -s -X POST https://api.porkbun.com/api/json/v3/pricing/get -H "Content-Type: application/json" -d '{}' | jq '.pricing | to_entries[] | select(.value.registration | tonumber < 15) | {tld: .key, reg: .value.registration, renewal: .value.renewal}'`
  - This returns ALL TLDs Porkbun sells with registration and renewal prices
  - Filter by price to find cheap options: `| tonumber < 10` for under $10/year

## Full TLD List

Use `domain-check <name> --all` to check against all ~1,200 known TLDs.

**Exclude non-purchasable TLDs** — `.gov`, `.mil`, `.edu`, `.int` and many ccTLDs require residency or special eligibility. The `domain-check` tool handles this via RDAP — non-purchasable TLDs will show as "unavailable" or "error" rather than "available".

## Choosing TLDs

General guidance for the skill to follow:

- **`.com`** — still the default. Always check it. If available and the name is good, this is almost always the right pick.
- **`.dev`** — Google-owned, HTTPS-enforced, developer-focused. Strong choice for dev tools/libraries.
- **`.sh`** — Popular for CLI tools and shell utilities. Short.
- **`.io`** — Historically popular for startups/tech. Higher renewal costs. Some controversy over British Indian Ocean Territory origins.
- **`.app`** — Google-owned, HTTPS-enforced. Good for application projects.
- **`.tools`** / `.codes` / `.software`** — Descriptive, cheap, but longer.
- **`.ai`** — Expensive (~$50+/year), trendy. Only if the project is genuinely AI-related.
- **`.rs`** — Officially Serbia's ccTLD but commonly used for Rust projects. Cheap.
- **`.run`** — Good for runners, CLI tools, automation.
- **`.land`** — Quirky, memorable. Good for creative projects.
- **`.xyz`** — Cheap, no connotation. Good as a budget fallback.
- **`.moe`** — Japanese slang for cute/adorable obsession. Perfect for anime/manga/otaku projects. ⚠️ RDAP server is broken (returns 404 for everything) — always verify results with WHOIS/DNS.
- **`.ink`** — Great for manga, comics, writing, illustration projects. Short.
- **`.lol`** / **`.wtf`** — Edgy, humorous, irreverent. Good for meme-adjacent or provocative projects.
- **`.fan`** — Fan communities, fandom projects.

**Think thematically.** Don't just check tech TLDs for every project. If the project is anime-adjacent, check `.moe`, `.ink`, `.art`. If it's edgy/provocative, check `.wtf`, `.lol`. Match TLDs to the project's culture.
