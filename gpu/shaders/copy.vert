#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

void main() {
	switch (gl_VertexIndex) {
	case 0:
		gl_Position = vec4(-1.0, +1.0, 0.0, 1.0);
		break;
	case 1:
		gl_Position = vec4(+1.0, +1.0, 0.0, 1.0);
		break;
	case 2:
		gl_Position = vec4(-1.0, -1.0, 0.0, 1.0);
		break;
	case 3:
		gl_Position = vec4(+1.0, -1.0, 0.0, 1.0);
		break;
	}
}
