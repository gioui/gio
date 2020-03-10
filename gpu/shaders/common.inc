// SPDX-License-Identifier: Unlicense OR MIT

// fboTextureTransform returns a transformation
// that cancels the implied transformation between
// the framebuffer and its texture.
// Only two rows are returned. The last is implied
// to be [0, 0, 1].
vec3[2] fboTextureTransform() {
	vec3[2] t;
#ifdef HLSL
	t[0] = vec3(1.0, 0.0, 0.0);
	t[1] = vec3(0.0, -1.0, 1.0);
#else
	t[0] = vec3(1.0, 0.0, 0.0);
	t[1] = vec3(0.0, 1.0, 0.0);
#endif
	return t;
}

// fboTransform returns a transformation
// that cancels the implied transformation between
// the clip space and the framebuffer.
// Only two rows are returned. The last is implied
// to be [0, 0, 1].
vec3[2] fboTransform() {
	vec3[2] t;
#ifdef HLSL
	t[0] = vec3(1.0, 0.0, 0.0);
	t[1] = vec3(0.0, 1.0, 0.0);
#else
	t[0] = vec3(1.0, 0.0, 0.0);
	t[1] = vec3(0.0, -1.0, 0.0);
#endif
	return t;
}

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

vec3 transform3x2(vec3[2] t, vec3 v) {
	return vec3(dot(t[0], v), dot(t[1], v), dot(vec3(0.0, 0.0, 1.0), v));
}
