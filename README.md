# OwnPulse Public Site

Public-facing website for OwnPulse at [ownpulse.health](https://ownpulse.health). Landing page, about page, and waitlist. Not part of the application — self-hosters do not need this.

## Stack

- [Astro 4](https://astro.build) — static site generator
- [Tailwind CSS](https://tailwindcss.com) — utility-first CSS
- nginx — serving the static build in production

## Development

```bash
npm install
npm run dev      # http://localhost:4321
npm run build    # produces dist/
npm run preview  # preview the build locally
```

## Deployment

Deployed as an nginx container in the k3s cluster via the Helm chart in `helm/web-public/`.

```bash
docker build -t ownpulse-web-public .
helm upgrade --install web-public helm/web-public -n ownpulse
```

## License

AGPL-3.0-or-later
