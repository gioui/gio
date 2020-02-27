// SPDX-License-Identifier: Unlicense OR MIT

// fboTextureTransform returns a transformation
// that cancels the implied transformation between
// the framebuffer and its texture.
// Only two rows are returned. The last is implied
// to be [0, 0, 1].
vec3[2] fboTextureTransform() {
	vec3[2] t;
	t[0] = vec3(1.0, 0.0, 0.0);
	t[1] = vec3(0.0, 1.0, 0.0);
	return t;
}

vec3 transform3x2(vec3[2] t, vec3 v) {
	return vec3(dot(t[0], v), dot(t[1], v), dot(vec3(0.0, 0.0, 1.0), v));
}
