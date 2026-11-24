# avatar-pack

The household's avatar library as a versioned pack: every picture the avatars service deals
to at-claude sessions, with its name, handles and crop. The pictures are not built into the
service. Each ecosystem (home, and each work laptop on its own) runs the same avatars service,
and every one of them loads this pack by itself from its public GitHub copy
([ATC-192](https://openproject.home.romaine.life/wp/ATC-192); this repo is
[AVATARS-3](https://openproject.home.romaine.life/wp/AVATARS-3)).

- Forgejo, https://forgejo.romaine.life/nelsong6/avatar-pack, is the source: pull requests
  and CI are here, and merge-queue lands them.
- GitHub, https://github.com/nelsong6/avatar-pack, is a public copy that a Forgejo push
  mirror updates on every commit. Work laptops reach only GitHub, so every service reads the
  pack from there, home's included. Nothing is ever changed on GitHub directly.

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

- `kind` is `agent` (Claude's messages), `system` (system events) or `personal` (Nelson's
  own, never dealt; at most one).
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
  exists, matches its `sha256` and decodes as PNG; kinds are agent, system or personal, with
  at most one personal; every agent has at least one handle; a name is 1 to 80 characters.
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

## How services pick it up

Each avatars service asks GitHub for `main`'s head
(`GET https://api.github.com/repos/nelsong6/avatar-pack/commits/main` with its last ETag, so an
unchanged answer is a free 304) when it starts and then hourly. When the sha differs from the
one it last loaded, it downloads `https://codeload.github.com/nelsong6/avatar-pack/tar.gz/<sha>`,
checks every rule above, and imports the pack in one transaction: upsert by id, mark ids no
longer listed as deleted, record the sha. A pack whose `format` it doesn't know is refused and
logged, never misread. So a merge here reaches every ecosystem within the hour.

## Shipping

Nothing ships from here: there is no build and no release. A merge is delivered once the
mirror has updated GitHub, which it does as the merge lands.
