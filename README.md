# OSM Tile Generator (Go)

Generates OSM-compatible slippy map tiles (`{z}/{x}/{y}.png`) from a single
georeferenced image. You provide an image and four control points that map
pixel coordinates in the image to real-world lat/lng, and the tool warps the
image via homography and slices it into tiles across a range of zoom levels.

## Usage

The input can be created with [Georeferencer-WebUI](https://github.com/Nils-witt/Georeferencer-WebUI)

Install the CLI, or download a binary from the GitHub releases:

```sh
go install github.com/Nils-witt/OSM_TileGenerator_GO/cmd/osm-tile-generator@latest
```

Generate tiles, then serve them locally:

```sh
osm-tile-generator generate -config-dir config -min-zoom 14 -max-zoom 18
osm-tile-generator serve -config-dir config -port 8080
```

| Command    | Description                                                 |
|------------|-------------------------------------------------------------|
| `generate` | Warp `<config-dir>/image.png` into a tile pyramid and zip it |
| `serve`    | Serve `<config-dir>/tiles` over HTTP with CORS enabled      |
| `version`  | Print version information and the project URL               |

Run `osm-tile-generator <command> -h` to list a command's flags.

### `generate` flags

| Flag          | Default  | Description                           |
|---------------|----------|---------------------------------------|
| `-config-dir` | `config` | Directory containing config and image |
| `-min-zoom`   | `14`     | Minimum zoom level to generate        |
| `-max-zoom`   | `18`     | Maximum zoom level to generate        |
| `-log-level`  | `info`   | `debug`, `info`, `warn` or `error`    |

### `serve` flags

| Flag          | Default  | Description                              |
|---------------|----------|------------------------------------------|
| `-config-dir` | `config` | Directory whose `tiles/` folder is served |
| `-port`       | `8080`   | TCP port to listen on                    |
| `-log-level`  | `info`   | `debug`, `info`, `warn` or `error`       |

Running without a command (`osm-tile-generator -config-dir config`, or
`-serve <port>`) still works but is deprecated and prints a warning.

Progress is logged to stderr as structured `key=value` lines; stdout is only
used for command output such as `version`. Use `-log-level debug` to see
per-marker and per-tile details, or `-log-level warn` for quiet runs.

Invalid flags exit with status 2; runtime failures exit with status 1.

### Input directory layout

The config directory must contain:

- `config.json` — project metadata and control points (see below)
- `image.png` — the source image to be tiled

```json
{
  "img_scale": { "width": 1024, "height": 768 },
  "coordinates": [
    { "img": { "x": 0, "y": 0 }, "map": { "latitude": 0.0, "longitude": 0.0 } },
    { "img": { "x": 1024, "y": 0 }, "map": { "latitude": 0.0, "longitude": 0.0 } },
    { "img": { "x": 1024, "y": 768 }, "map": { "latitude": 0.0, "longitude": 0.0 } },
    { "img": { "x": 0, "y": 768 }, "map": { "latitude": 0.0, "longitude": 0.0 } }
  ]
}
```

`img_scale` should match the displayed pixel size of the image when the
control points were picked (e.g. in a web UI), so pixel coordinates can be
rescaled to the source image's true dimensions. Exactly four coordinate
pairs are required.

### Output

Tiles are written to `<config-dir>/tiles/{z}/{x}/{y}.png`, generated at
`-max-zoom` and downsampled for each lower zoom level down to `-min-zoom`.
A `<config-dir>/tmp` directory is used for intermediate files, and the
finished pyramid is also archived as `<config-dir>/tiles.zip`.

## Requirements

- Go 1.26+
