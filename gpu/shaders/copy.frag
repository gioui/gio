#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

layout(location = 0) out vec4 fragColor;

layout(binding = 0) uniform sampler2D tex;

precision mediump float;

vec3 sRGBtoRGB(vec3 rgb) {
	bvec3 cutoff = greaterThanEqual(rgb, vec3(0.04045));
	vec3 below = rgb/vec3(12.92);
	vec3 above = pow((rgb + vec3(0.055))/vec3(1.055), vec3(2.4));
	return mix(below, above, cutoff);
}

void main() {
	vec4 texel = texelFetch(tex, ivec2(gl_FragCoord.xy), 0);
	vec3 rgb = sRGBtoRGB(texel.rgb);
	fragColor = vec4(rgb, texel.a);
}
