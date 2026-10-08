// SPDX-License-Identifier: Unlicense OR MIT

package driver

import "image"

// ExternalTexture describes a texture whose contents are managed outside of
// Gio, for example by a custom rendering pass. Painting it does not upload any
// pixels: the blitter samples the existing GPU resource.
type ExternalTexture struct {
	// Handle is a backend specific resource. For Direct3D 11 it is a
	// *d3d11.Texture2D.
	Handle any
	// Format is the backend's texture format enum.
	Format TextureFormat
	// Bounds is the size of the top mip level in pixels.
	Bounds image.Point
	// Mip is the number of mip levels, or 0 if the texture has none.
	Mip int
	// RenderTarget is true if the resource can be used as a render target.
	RenderTarget bool
	// MinFilter and MagFilter are the samplers to create for the texture.
	MinFilter, MagFilter TextureFilter
}

// ExternalTextureDevice is implemented by backends that can adopt a texture
// created outside of Gio. Backends that cannot leave it unimplemented, and
// the renderer falls back to uploading CPU pixels.
type ExternalTextureDevice interface {
	NewExternalTexture(src ExternalTexture) (Texture, error)
}
