// SPDX-License-Identifier: Unlicense OR MIT

/*
Package paint provides operations for 2D graphics.

The PaintOp operation draws the current material into a rectangular
area, taking the current clip path and transformation into account.

The material is set by either a ColorOp for a constant color, or
ImageOp for an image.

The ClipOp operation sets the clip path. Drawing outside the clip
path is ignored. A path is a closed shape of lines or curves.
*/
package paint
