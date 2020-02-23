#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

void main() {
	float x, y;
	if (gl_VertexIndex == 0) {
		x = 0.0;
		y = .5;
	} else if (gl_VertexIndex == 1) {
		x = .5;
		y = -.5;
	} else {
		x = -.5;
		y = -.5;
	}
	gl_Position = vec4(x, y, 0.5, 1.0);
}
