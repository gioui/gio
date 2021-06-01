// SPDX-License-Identifier: Unlicense OR MIT

package gl

type (
	Attrib uint
	Enum   uint
)

const (
	ALL_BARRIER_BITS                      = 0xffffffff
	ARRAY_BUFFER                          = 0x8892
	BACK                                  = 0x0405
	BLEND                                 = 0xbe2
	CLAMP_TO_EDGE                         = 0x812f
	COLOR_ATTACHMENT0                     = 0x8ce0
	COLOR_BUFFER_BIT                      = 0x4000
	COMPILE_STATUS                        = 0x8b81
	COMPUTE_SHADER                        = 0x91B9
	DEPTH_BUFFER_BIT                      = 0x100
	DEPTH_ATTACHMENT                      = 0x8d00
	DEPTH_COMPONENT16                     = 0x81a5
	DEPTH_COMPONENT24                     = 0x81A6
	DEPTH_COMPONENT32F                    = 0x8CAC
	DEPTH_TEST                            = 0xb71
	DRAW_FRAMEBUFFER                      = 0x8CA9
	DST_COLOR                             = 0x306
	DYNAMIC_DRAW                          = 0x88E8
	DYNAMIC_READ                          = 0x88E9
	ELEMENT_ARRAY_BUFFER                  = 0x8893
	EXTENSIONS                            = 0x1f03
	FALSE                                 = 0
	FLOAT                                 = 0x1406
	FRAGMENT_SHADER                       = 0x8b30
	FRAMEBUFFER                           = 0x8d40
	FRAMEBUFFER_ATTACHMENT_COLOR_ENCODING = 0x8210
	FRAMEBUFFER_BINDING                   = 0x8ca6
	FRAMEBUFFER_COMPLETE                  = 0x8cd5
	FRAMEBUFFER_SRGB                      = 0x8db9
	HALF_FLOAT                            = 0x140b
	HALF_FLOAT_OES                        = 0x8d61
	INFO_LOG_LENGTH                       = 0x8B84
	INVALID_INDEX                         = ^uint(0)
	GREATER                               = 0x204
	GEQUAL                                = 0x206
	LINEAR                                = 0x2601
	LINK_STATUS                           = 0x8b82
	LUMINANCE                             = 0x1909
	MAP_READ_BIT                          = 0x0001
	MAX_TEXTURE_SIZE                      = 0xd33
	NEAREST                               = 0x2600
	NO_ERROR                              = 0x0
	NUM_EXTENSIONS                        = 0x821D
	ONE                                   = 0x1
	ONE_MINUS_SRC_ALPHA                   = 0x303
	PROGRAM_BINARY_LENGTH                 = 0x8741
	QUERY_RESULT                          = 0x8866
	QUERY_RESULT_AVAILABLE                = 0x8867
	R16F                                  = 0x822d
	R8                                    = 0x8229
	READ_FRAMEBUFFER                      = 0x8ca8
	READ_ONLY                             = 0x88B8
	READ_WRITE                            = 0x88BA
	RED                                   = 0x1903
	RENDERER                              = 0x1F01
	RENDERBUFFER                          = 0x8d41
	RENDERBUFFER_BINDING                  = 0x8ca7
	RENDERBUFFER_HEIGHT                   = 0x8d43
	RENDERBUFFER_WIDTH                    = 0x8d42
	RGB                                   = 0x1907
	RGBA                                  = 0x1908
	RGBA8                                 = 0x8058
	SHADER_STORAGE_BUFFER                 = 0x90D2
	SHORT                                 = 0x1402
	SRGB                                  = 0x8c40
	SRGB_ALPHA_EXT                        = 0x8c42
	SRGB8                                 = 0x8c41
	SRGB8_ALPHA8                          = 0x8c43
	STATIC_DRAW                           = 0x88e4
	STENCIL_BUFFER_BIT                    = 0x00000400
	TEXTURE_2D                            = 0xde1
	TEXTURE_MAG_FILTER                    = 0x2800
	TEXTURE_MIN_FILTER                    = 0x2801
	TEXTURE_WRAP_S                        = 0x2802
	TEXTURE_WRAP_T                        = 0x2803
	TEXTURE0                              = 0x84c0
	TEXTURE1                              = 0x84c1
	TRIANGLE_STRIP                        = 0x5
	TRIANGLES                             = 0x4
	TRUE                                  = 1
	UNIFORM_BUFFER                        = 0x8A11
	UNPACK_ALIGNMENT                      = 0xcf5
	UNSIGNED_BYTE                         = 0x1401
	UNSIGNED_SHORT                        = 0x1403
	VERSION                               = 0x1f02
	VERTEX_SHADER                         = 0x8b31
	WRITE_ONLY                            = 0x88B9
	ZERO                                  = 0x0

	// EXT_disjoint_timer_query
	TIME_ELAPSED_EXT = 0x88BF
	GPU_DISJOINT_EXT = 0x8FBB
)
