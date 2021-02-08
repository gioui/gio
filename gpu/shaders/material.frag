#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision mediump float;

layout(binding = 0) uniform sampler2D tex;

layout(location = 0) in vec2 vUV;

layout(location = 0) out vec4 fragColor;

vec3 RGBtosRGB(vec3 rgb) {
	bvec3 cutoff = greaterThanEqual(rgb, vec3(0.0031308));
	vec3 below = vec3(12.92)*rgb;
	vec3 above = vec3(1.055)*pow(rgb, vec3(0.41666)) - vec3(0.055);
	return mix(below, above, cutoff);
}

void main() {
	vec4 texel = texture(tex, vUV);
	texel.rgb = RGBtosRGB(texel.rgb);
	fragColor = texel;
}
