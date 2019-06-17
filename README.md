# Optimg

## Description

Use discord app project [lilliput](https://github.com/discordapp/lilliput) Go bindings to optimize images for the web. The motive for creating this project is to come up with a photographer's "finishing" tool, to prepare images for export to the web or to clients.  

## Todo

- [x] Add `pctResize` cli parameter to automatically scale width/height
- [x] Add `force` cli parameter to overwrite existing file
- [ ] Add ability to edit/remove exif data
- [ ] Add watermarking
  - Detect pixel regions where watermarks can be safely added

## Attribution

Forked from a simple [example](https://github.com/discordapp/lilliput/blob/master/examples/main.go) in the [lilliput](https://github.com/discordapp/lilliput) repo.
