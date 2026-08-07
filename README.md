# OSM Tile Generator (Go)

Generates OSM-compatible slippy map tiles (`{z}/{x}/{y}.png`) from a single
georeferenced image. You provide an image and four control points that map
pixel coordinates in the image to real-world lat/lng, and the tool warps the
image via homography and slices it into tiles across a range of zoom levels.

## Usage

```sh
go run ./src --config-dir config --min-zoom 14 --max-zoom 18
```

### Flags

| Flag          | Default  | Description                              |
|---------------|----------|-------------------------------------------|
| `-config-dir` | `config` | Directory containing config and image     |
| `-min-zoom`   | `14`     | Minimum zoom level to generate            |
| `-max-zoom`   | `18`     | Maximum zoom level to generate            |

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
A `<config-dir>/tmp` directory is used for intermediate files.

## Requirements

- Go 1.26+
