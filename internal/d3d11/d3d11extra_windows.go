// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

// Formats and flags that the renderer's own pipelines don't need, but custom
// rendering passes do. Values are from dxgiformat.h and d3d11.h.
const (
	DXGI_FORMAT_D32_FLOAT  = 40
	DXGI_FORMAT_R8G8_UNORM = 49
	DXGI_FORMAT_R8_UNORM   = 61

	// Block compressed formats, the layout textures are stored in inside
	// most game archives.
	DXGI_FORMAT_BC1_UNORM      = 71
	DXGI_FORMAT_BC1_UNORM_SRGB = 72
	DXGI_FORMAT_BC2_UNORM      = 74
	DXGI_FORMAT_BC2_UNORM_SRGB = 75
	DXGI_FORMAT_BC3_UNORM      = 77
	DXGI_FORMAT_BC3_UNORM_SRGB = 78
	DXGI_FORMAT_BC4_UNORM      = 80
	DXGI_FORMAT_BC4_SNORM      = 81
	DXGI_FORMAT_BC5_UNORM      = 83
	DXGI_FORMAT_BC5_SNORM      = 84
	DXGI_FORMAT_BC7_UNORM      = 98
	DXGI_FORMAT_BC7_UNORM_SRGB = 99

	USAGE_DEFAULT    = 0
	USAGE_DYNAMIC    = 2
	CPU_ACCESS_WRITE = 0x10000

	MAP_WRITE         = 2
	MAP_WRITE_DISCARD = 4

	CULL_FRONT = 2
	CULL_BACK  = 3

	DEPTH_COMPARISON_NEVER      = 1
	DEPTH_COMPARISON_LESS       = 2
	DEPTH_COMPARISON_LESS_EQUAL = 4
)

// BlockSize returns the number of bytes one mip level of a block compressed
// format occupies, or 0 for uncompressed formats. It returns an error for
// dimensions that are too small to be encoded in the format.
func BlockSize(format uint32, width, height int) (int, bool) {
	var blockBytes, blockWidth, blockHeight int
	switch format {
	case DXGI_FORMAT_BC1_UNORM, DXGI_FORMAT_BC1_UNORM_SRGB:
		blockBytes, blockWidth, blockHeight = 8, 4, 4
	case DXGI_FORMAT_BC2_UNORM, DXGI_FORMAT_BC2_UNORM_SRGB,
		DXGI_FORMAT_BC3_UNORM, DXGI_FORMAT_BC3_UNORM_SRGB:
		blockBytes, blockWidth, blockHeight = 16, 4, 4
	case DXGI_FORMAT_BC4_UNORM, DXGI_FORMAT_BC4_SNORM,
		DXGI_FORMAT_BC5_UNORM, DXGI_FORMAT_BC5_SNORM,
		DXGI_FORMAT_BC7_UNORM, DXGI_FORMAT_BC7_UNORM_SRGB:
		blockBytes, blockWidth, blockHeight = 16, 4, 4
	default:
		return 0, false
	}
	blocksWide := (width + blockWidth - 1) / blockWidth
	blocksHigh := (height + blockHeight - 1) / blockHeight
	return blocksWide * blocksHigh * blockBytes, true
}

// BlockRowSize returns the number of bytes in one row of a block compressed
// mip level, or 0 for uncompressed formats.
func BlockRowSize(format uint32, width int) (int, bool) {
	var blockBytes, blockWidth int
	switch format {
	case DXGI_FORMAT_BC1_UNORM, DXGI_FORMAT_BC1_UNORM_SRGB:
		blockBytes, blockWidth = 8, 4
	case DXGI_FORMAT_BC2_UNORM, DXGI_FORMAT_BC2_UNORM_SRGB,
		DXGI_FORMAT_BC3_UNORM, DXGI_FORMAT_BC3_UNORM_SRGB,
		DXGI_FORMAT_BC4_UNORM, DXGI_FORMAT_BC4_SNORM,
		DXGI_FORMAT_BC5_UNORM, DXGI_FORMAT_BC5_SNORM,
		DXGI_FORMAT_BC7_UNORM, DXGI_FORMAT_BC7_UNORM_SRGB:
		blockBytes, blockWidth = 16, 4
	default:
		return 0, false
	}
	return ((width + blockWidth - 1) / blockWidth) * blockBytes, true
}
