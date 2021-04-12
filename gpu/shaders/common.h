// SPDX-License-Identifier: Unlicense OR MIT

struct m3x2 {
	vec3 r0;
	vec3 r1;
};

// fboTextureTransform is the transformation
// that cancels the implied transformation between
// the framebuffer and its texture.
// Only two rows are returned. The last is implied
// to be [0, 0, 1].
const m3x2 fboTextureTransform = m3x2(
#ifdef HLSL
	vec3(1.0, 0.0, 0.0),
	vec3(0.0, -1.0, 1.0)
#else
	vec3(1.0, 0.0, 0.0),
	vec3(0.0, 1.0, 0.0)
#endif
);

// fboTransform is the transformation
// that cancels the implied transformation between
// the clip space and the framebuffer.
// Only two rows are returned. The last is implied
// to be [0, 0, 1].
const m3x2 fboTransform = m3x2(
#ifdef HLSL
	vec3(1.0, 0.0, 0.0),
	vec3(0.0, 1.0, 0.0)
#else
	vec3(1.0, 0.0, 0.0),
	vec3(0.0, -1.0, 0.0)
#endif
);

// toClipSpace converts an OpenGL gl_Position value to a
// native GPU position.
vec4 toClipSpace(vec4 pos) {
#ifdef HLSL
	// Map depths to the Direct3D [0; 1] range.
	return vec4(pos.xy, (pos.z + pos.w)*.5, pos.w);
#else
	return pos;
#endif
}

vec3 transform3x2(m3x2 t, vec3 v) {
	return vec3(dot(t.r0, v), dot(t.r1, v), dot(vec3(0.0, 0.0, 1.0), v));
}
