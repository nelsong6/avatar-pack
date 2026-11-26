# avatar-pack

The household's avatar library as a versioned pack: every picture the avatars service deals
to at-claude sessions, with its name, handles and crop. The pictures are not built into the
service. Each ecosystem (home, and each work laptop on its own) runs the same avatars service,
and every one of them loads this pack by itself: home's from its releases on Forgejo, each work
laptop's from its public GitHub copy ([ATC-192](https://openproject.home.romaine.life/wp/ATC-192);
this repo is [AVATARS-3](https://openproject.home.romaine.life/wp/AVATARS-3), its releases
[AVATARS-6](https://openproject.home.romaine.life/wp/AVATARS-6)).

- Forgejo, https://forgejo.romaine.life/nelsong6/avatar-pack, is the source: pull requests,
  CI and releases are here, and merge-queue lands them.
- GitHub, https://github.com/nelsong6/avatar-pack, is a public copy that a Forgejo push
  mirror updates on every commit and release. It exists only for the work laptops, which
  can't reach home's services (ATC-192). Nothing is ever changed on GitHub directly.

The library was exported once from home's live service (`GET /avatars` and each
`/avatars/{id}/original`, every file checked against the SHA-256 the service gives it). From
here on this repo is the library: a change to it is a pull request.

## Format 1

`pack.json`:

```json
{
  "format": 1,
  "avatars": [
    {
      "id": "jp1-raptor",
      "kind": "agent",
      "name": "Velociraptor",
      "handles": ["raptor"],
      "crop": {"size": 1, "center_x": 0.5, "center_y": 0.5},
      "original": "originals/jp1-raptor.png",
      "sha256": "9ff90ff8..."
    }
  ]
}
```

- `kind` is `agent` (Claude's messages), `system` (system events), `personal` (Nelson's
  own, never dealt; at most one) or `reviewer` (the architecture reviewer's, beside a
  review's end in at-claude, ATC-205; never dealt; at most one).
- `handles` are an agent's gamertag names (`pensive-raptor`): lowercase letters and digits,
  at least one for every agent.
- `crop` is where the 512x512 circle sits on the original, as the service stores it: the
  circle's centre as a fraction of the original's width and height (`center_x`, `center_y`),
  and its diameter as a fraction of the shorter side (`size`). The service cuts the circle
  from the original with it (avatars' `circle.go`).
- `original` is always `originals/<id>.png`, and `sha256` is the hex SHA-256 of that file.
- The list is in library order: by kind, oldest first. A service gives new avatars that
  order, so a new one goes at the end of its kind.

## The rules

CI (`cmd/check`, in `.forgejo/workflows/ci.yml`) holds every pull request to them, comparing
with the `main` it is based on; the go suite (pipeline-templates' `fleet.json`) formats, vets
and tests `cmd/check` itself:

- An id's picture never changes. at-claude caches pictures by id forever, so an id already on
  `main` keeps its `sha256` and its `crop`. A changed picture, or a re-centred one, is a new id.
- `format` is 1; ids are unique, 1 to 100 letters, digits, `-` and `_` (they go in URLs); every listed file
  exists, matches its `sha256` and decodes as PNG; kinds are agent, system, personal or reviewer,
  with at most one personal and one reviewer; every agent has at least one handle; a name is 1 to 80 characters.
- The pack holds at least one agent and at least one system avatar. A service refuses a pack
  without them, since loading it would mark every avatar of that kind it holds deleted.
- Every file in `originals/` is listed, so a removed avatar takes its picture with it.

Run them locally (four cores, as CI does):

```
GOMAXPROCS=4 go test -p 4 ./...
GOMAXPROCS=4 go run ./cmd/check -base origin/main
```

## Changing the library

- **Add:** put the picture at `originals/<new id>.png`, and add an entry with a new id (any
  unused id; new ones are `av_` and 24 hex digits, as the service made them), its kind, name,
  handles, crop and the file's `sha256` at the end of its kind.
- **Replace or re-crop a picture:** that is a new avatar. Add it with a new id and remove the
  old one; never change an existing id's file, `sha256` or `crop`.
- **Rename:** change `name` or `handles` in place. The id stays, so sessions keep their
  picture.
- **Remove:** delete the entry and its file. A service keeps the images it already holds,
  marked deleted, so sessions dealt it still show it.
- **Nelson's own avatar:** replace the `personal` entry with one under a new id.

## Shipping

A merge that changes the pack releases it: pipeline-templates' release suite (`release-module`
in its `fleet.json` entry) takes the next `vX.Y.Z` tag and publishes a Forgejo release of the
merge's source, building nothing. A merge changing only documentation, tests or workflows
releases nothing.

Home's avatars service loads each release when Forgejo announces it: it reads the pack at the
release's commit, checks every rule above, and imports it in one transaction (upsert by id,
mark ids no longer listed as deleted, record the commit). A pack whose `format` it doesn't
know is refused, never misread. It then reports on ntfy's `activity` topic as `avatars/home`,
`applied` or `failed` with the release's commit. The release is delivered when that report
names it as applied: strip-owner (agent-tools) waits for it, since `fleet.json` names
`avatars/home` as this repo's applier, and a `failed` report wakes the session that merged it.

Each work laptop's avatars service can't reach home, so it keeps reading the GitHub copy: it
asks GitHub for `main`'s head
(`GET https://api.github.com/repos/nelsong6/avatar-pack/commits/main` with its last ETag, so an
unchanged answer is a free 304) when it starts and then hourly, and when the sha differs from
the one it last loaded it downloads `https://codeload.github.com/nelsong6/avatar-pack/tar.gz/<sha>`
and imports it the same way. A merge reaches the work laptops within the hour, with no report
back home.
