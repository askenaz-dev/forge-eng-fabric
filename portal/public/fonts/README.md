# Self-hosted Forge Portal fonts

This directory holds the three brand families used by the Portal. **No Google
Fonts or external CDN requests are issued at runtime.**

## Required files (WOFF2)

Place the following binaries here before building. Each file is referenced
explicitly by `portal/src/app/fonts.ts` via `next/font/local`.

| Family             | Weight | Style  | Filename                            | License  |
|--------------------|--------|--------|-------------------------------------|----------|
| Instrument Serif   | 400    | normal | `instrument-serif-regular.woff2`    | OFL 1.1  |
| Instrument Serif   | 400    | italic | `instrument-serif-italic.woff2`     | OFL 1.1  |
| Geist              | 400    | normal | `geist-regular.woff2`               | OFL 1.1  |
| Geist              | 500    | normal | `geist-medium.woff2`                | OFL 1.1  |
| Geist              | 600    | normal | `geist-semibold.woff2`              | OFL 1.1  |
| JetBrains Mono     | 400    | normal | `jetbrains-mono-regular.woff2`      | OFL 1.1  |
| JetBrains Mono     | 400    | italic | `jetbrains-mono-italic.woff2`       | OFL 1.1  |

All three families are SIL Open Font License — they may be redistributed in
binary form alongside the application. The license file SHOULD be included
when shipping a binary build (`portal/public/fonts/OFL.txt`).

Sources:
- Instrument Serif: https://github.com/Instrument/instrument-serif
- Geist:            https://github.com/vercel/geist-font
- JetBrains Mono:   https://github.com/JetBrains/JetBrainsMono

## Verifying

After placing the files, run the Portal in dev mode and confirm that the
Network panel shows the font requests served from `/fonts/...` and that no
request is made to `fonts.googleapis.com` or `fonts.gstatic.com`.
