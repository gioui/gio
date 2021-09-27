(() => {

    // webgl is the array which handles the WebGL context. See InitGL function.
    let webgl = 0;

    // textDecoder holds the TextDecoder used for encode string.
    let textDecoder = new TextDecoder("utf-8");

    // invalidateBuffer is re-use when you call invalidateBuffer().
    let invalidateBuffer = new Int32Array(1);

    // hold values from JS
    let values = [undefined]
    let valuesPool = []

    // Offset* is the byte-size of each type (matches with Reflect.Sizeof()).
    const OffsetContextIndex = 8;
    const OffsetInt64 = 8;
    const OffsetFloat64 = 8;
    const OffsetJSValue = 8;
    const OffsetString = 16;
    const OffsetSlice = 24;

	globalThis.setUnsafeGL = (gl) => {
        webgl = gl
    }

    const gioLoadBool = (addr) => {
        return gioLoadInt64(addr) > 0
    }
    const gioLoadInt64 = (addr) => {
        return go.mem.getUint32(addr + 8, true) + go.mem.getInt32(addr + 12, true) * 4294967296;
    }
    const gioLoadInt32 = (addr) => {
        return go.mem.getUint32(addr + 8, true);
    }
    const gioLoadObject = (addr) => {
        return values[gioLoadInt64(addr)];
    }
    const gioLoadString = (addr) => {
        return textDecoder.decode(new DataView(go._inst.exports.mem.buffer, gioLoadInt64(addr), gioLoadInt64(addr + 8)));
    }
    const gioLoadSlice = (addr) => {
        const s = new Uint8Array(go._inst.exports.mem.buffer, gioLoadInt64(addr), gioLoadInt64(addr + 8))
        if (s.byteLength === 0) {
            return null
        }
        return s
    }
    const gioLoadFloat64 = (addr) => {
        return go.mem.getFloat64(addr + 8, true);
    }
    const gioLoadFloat32 = (addr) => {
        return go.mem.getFloat32(addr + 4, true);
    }

    const gioSetObject = (addr, v) => {
        let id = 0;
        if (v !== undefined && v !== null && v !== false) {
            id = valuesPool.pop();
            if (id !== undefined) {
                values[id] = v;
            } else {
                id = values.push(v) - 1;
            }
        }

        gioSetInt64(addr, id)
    }
    const gioSetInt64 = (addr, v) => {
		if (v === true) {
			v = 1;
		}
        go.mem.setUint32(addr + 8 + 4, 0, true);
        go.mem.setUint32(addr + 8, v, true);
    }
    const gioSetArray4 = (addr, r) => {
        for (let i = 0; i < r.length; i++) {
            gioSetInt64(addr, r[i])
            addr += 8
        }
    }

	const gioDeleteObject = (addr) => {
		valuesPool.push(gioLoadInt64(addr));
	}

    Object.assign(go.importObject.go, {
         // (f *FunctionCaller) ActiveTexture(t Enum)
		 "gioui.org/internal/gl.asmActiveTexture": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.activeTexture(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) AttachShader(p Program, s Shader)
		 "gioui.org/internal/gl.asmAttachShader": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.attachShader(
				gioLoadObject((sp)+0),
				gioLoadObject((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) BindAttribLocation(p Program, a Attrib, name string)
		 "gioui.org/internal/gl.asmBindAttribLocation": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bindAttribLocation(
				gioLoadObject((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadString((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) BindBuffer(target Enum, b Buffer)
		 "gioui.org/internal/gl.asmBindBuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bindBuffer(
				gioLoadInt64((sp)+0),
				gioLoadObject((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) BindBufferBase(target Enum, index int, b Buffer)
		 "gioui.org/internal/gl.asmBindBufferBase": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bindBufferBase(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadObject((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) BindFramebuffer(target Enum, fb Framebuffer)
		 "gioui.org/internal/gl.asmBindFramebuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bindFramebuffer(
				gioLoadInt64((sp)+0),
				gioLoadObject((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) BindRenderbuffer(target Enum, rb Renderbuffer)
		 "gioui.org/internal/gl.asmBindRenderbuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bindRenderbuffer(
				gioLoadInt64((sp)+0),
				gioLoadObject((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) BindTexture(target Enum, t Texture)
		 "gioui.org/internal/gl.asmBindTexture": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bindTexture(
				gioLoadInt64((sp)+0),
				gioLoadObject((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) BlendEquation(mode Enum)
		 "gioui.org/internal/gl.asmBlendEquation": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.blendEquation(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) BlendFuncSeparate(srcRGB, dstRGB, srcA, dstA Enum)
		 "gioui.org/internal/gl.asmBlendFuncSeparate": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.blendFunc(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
			);

            
        },
         // (f *FunctionCaller) BufferData(target Enum, usage Enum, data []byte)
		 "gioui.org/internal/gl.asmBufferData": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bufferData(
				gioLoadInt64((sp)+0),
				gioLoadSlice((sp)+0+8),
				gioLoadInt64((sp)+0+8+24),
			);

            
        },
         // (f *FunctionCaller) BufferDataSize(target Enum, size int, usage Enum)
		 "gioui.org/internal/gl.asmBufferDataSize": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bufferData(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) BufferSubData(target Enum, offset int, src []byte)
		 "gioui.org/internal/gl.asmBufferSubData": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.bufferSubData(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadSlice((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) CheckFramebufferStatus(target Enum) Enum
		 "gioui.org/internal/gl.asmCheckFramebufferStatus": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.checkFramebufferStatus(
				gioLoadInt64((sp)+0),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) Clear(mask Enum)
		 "gioui.org/internal/gl.asmClear": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.clear(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) ClearColor(red, green, blue, alpha float32)
		 "gioui.org/internal/gl.asmClearColor": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.clearColor(
				gioLoadFloat64((sp)+0),
				gioLoadFloat64((sp)+0+8),
				gioLoadFloat64((sp)+0+8+8),
				gioLoadFloat64((sp)+0+8+8+8),
			);

            
        },
         // (f *FunctionCaller) ClearDepthf(d float32)
		 "gioui.org/internal/gl.asmClearDepthf": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.clearDepth(
				gioLoadFloat64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) CompileShader(s Shader)
		 "gioui.org/internal/gl.asmCompileShader": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.compileShader(
				gioLoadObject((sp)+0),
			);

            
        },
         // (f *FunctionCaller) CopyTexSubImage2D(target Enum, level, xoffset, yoffset, x, y, width, height int)
		 "gioui.org/internal/gl.asmCopyTexSubImage2D": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.copyTexSubImage2D(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8+8+8),
			);

            
        },
         // (f *FunctionCaller) CreateBuffer() Buffer
		 "gioui.org/internal/gl.asmCreateBuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.createBuffer(
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0, r)
        },
         // (f *FunctionCaller) CreateFramebuffer() Framebuffer
		 "gioui.org/internal/gl.asmCreateFramebuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.createFramebuffer(
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0, r)
        },
         // (f *FunctionCaller) CreateProgram() Program
		 "gioui.org/internal/gl.asmCreateProgram": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.createProgram(
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0, r)
        },
         // (f *FunctionCaller) CreateRenderbuffer() Renderbuffer
		 "gioui.org/internal/gl.asmCreateRenderbuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.createRenderbuffer(
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0, r)
        },
         // (f *FunctionCaller) CreateShader(ty Enum) Shader
		 "gioui.org/internal/gl.asmCreateShader": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.createShader(
				gioLoadInt64((sp)+0),
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) CreateTexture() Texture
		 "gioui.org/internal/gl.asmCreateTexture": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.createTexture(
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0, r)
        },
         // (f *FunctionCaller) DeleteBuffer(v Buffer)
		 "gioui.org/internal/gl.asmDeleteBuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.deleteBuffer(
				gioLoadObject((sp)+0),
			);

            gioDeleteObject(sp)
        },
         // (f *FunctionCaller) DeleteFramebuffer(v Framebuffer)
		 "gioui.org/internal/gl.asmDeleteFramebuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.deleteFramebuffer(
				gioLoadObject((sp)+0),
			);

            gioDeleteObject(sp)
        },
         // (f *FunctionCaller) DeleteProgram(p Program)
		 "gioui.org/internal/gl.asmDeleteProgram": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.deleteProgram(
				gioLoadObject((sp)+0),
			);

            gioDeleteObject(sp)
        },
         // (f *FunctionCaller) DeleteShader(s Shader)
		 "gioui.org/internal/gl.asmDeleteShader": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.deleteShader(
				gioLoadObject((sp)+0),
			);

            gioDeleteObject(sp)
        },
         // (f *FunctionCaller) DeleteRenderbuffer(v Renderbuffer)
		 "gioui.org/internal/gl.asmDeleteRenderbuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.deleteRenderbuffer(
				gioLoadObject((sp)+0),
			);

            gioDeleteObject(sp)
        },
         // (f *FunctionCaller) DeleteTexture(v Texture)
		 "gioui.org/internal/gl.asmDeleteTexture": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.deleteTexture(
				gioLoadObject((sp)+0),
			);

            gioDeleteObject(sp)
        },
         // (f *FunctionCaller) DepthFunc(fn Enum)
		 "gioui.org/internal/gl.asmDepthFunc": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.depthFunc(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) DepthMask(mask bool)
		 "gioui.org/internal/gl.asmDepthMask": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.depthMask(
				gioLoadBool((sp)+0),
			);

            
        },
         // (f *FunctionCaller) DisableVertexAttribArray(a Attrib)
		 "gioui.org/internal/gl.asmDisableVertexAttribArray": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.disableVertexAttribArray(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) Disable(cap Enum)
		 "gioui.org/internal/gl.asmDisable": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.disable(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) DrawArrays(mode Enum, first, count int)
		 "gioui.org/internal/gl.asmDrawArrays": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.drawArrays(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) DrawElements(mode Enum, count int, ty Enum, offset int)
		 "gioui.org/internal/gl.asmDrawElements": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.drawElements(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
			);

            
        },
         // (f *FunctionCaller) Enable(cap Enum)
		 "gioui.org/internal/gl.asmEnable": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.enable(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) EnableVertexAttribArray(a Attrib)
		 "gioui.org/internal/gl.asmEnableVertexAttribArray": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.enableVertexAttribArray(
				gioLoadInt64((sp)+0),
			);

            
        },
         // (f *FunctionCaller) Finish()
		 "gioui.org/internal/gl.asmFinish": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.finish(
			);

            
        },
         // (f *FunctionCaller) Flush()
		 "gioui.org/internal/gl.asmFlush": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.flush(
			);

            
        },
         // (f *FunctionCaller) FramebufferRenderbuffer(target, attachment, renderbuffertarget Enum, renderbuffer Renderbuffer)
		 "gioui.org/internal/gl.asmFramebufferRenderbuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.framebufferRenderbuffer(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadObject((sp)+0+8+8+8),
			);

            
        },
         // (f *FunctionCaller) FramebufferTexture2D(target, attachment, texTarget Enum, t Texture, level int)
		 "gioui.org/internal/gl.asmFramebufferTexture2D": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.framebufferTexture2D(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadObject((sp)+0+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8),
			);

            
        },
         // (f *FunctionCaller) GetRenderbufferParameteri(target, pname Enum) int
		 "gioui.org/internal/gl.asmGetRenderbufferParameteri": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getRenderbufferParameteri(
				gioLoadInt64((sp)+0),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) GetFramebufferAttachmentParameteri(target, attachment, pname Enum) int
		 "gioui.org/internal/gl.asmGetFramebufferAttachmentParameteri": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getFramebufferAttachmentParameter(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8+8+8, r)
        },
         // (f *FunctionCaller) GetBinding(pname Enum) Object
		 "gioui.org/internal/gl.asmGetBinding": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getParameter(
				gioLoadInt64((sp)+0),
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) GetBindingi(pname Enum, idx int) Object
		 "gioui.org/internal/gl.asmGetBindingi": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getIndexedParameter(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0+8+8, r)
        },
         // (f *FunctionCaller) GetInteger(pname Enum) int
		 "gioui.org/internal/gl.asmGetInteger": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getParameter(
				gioLoadInt64((sp)+0),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) GetFloat(pname Enum) float32
		 "gioui.org/internal/gl.asmGetFloat": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getParameter(
				gioLoadInt64((sp)+0),
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) GetInteger4(pname Enum) [4]int
		 "gioui.org/internal/gl.asmGetInteger4": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getParameter(
				gioLoadInt64((sp)+0),
			);

            gioSetArray4((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) GetFloat4(pname Enum) [4]float32
		 "gioui.org/internal/gl.asmGetFloat4": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getParameter(
				gioLoadInt64((sp)+0),
			);

            gioSetArray4((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) GetProgrami(p Program, pname Enum) int
		 "gioui.org/internal/gl.asmGetProgrami": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getProgramParameter(
				gioLoadObject((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8+8, r)
        },
         // (f *FunctionCaller) GetShaderi(s Shader, pname Enum) int
		 "gioui.org/internal/gl.asmGetShaderi": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getShaderParameter(
				gioLoadObject((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8+8, r)
        },
         // (f *FunctionCaller) GetUniformBlockIndex(p Program, name string) uint
		 "gioui.org/internal/gl.asmGetUniformBlockIndex": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getUniformBlockIndex(
				gioLoadObject((sp)+0),
				gioLoadString((sp)+0+8),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8+16, r)
        },
         // (f *FunctionCaller) GetUniformLocation(p Program, name string) Uniform
		 "gioui.org/internal/gl.asmGetUniformLocation": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getUniformLocation(
				gioLoadObject((sp)+0),
				gioLoadString((sp)+0+8),
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0+8+16, r)
        },
         // (f *FunctionCaller) GetVertexAttrib(index int, pname Enum) int
		 "gioui.org/internal/gl.asmGetVertexAttrib": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getVertexAttrib(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8+8, r)
        },
         // (f *FunctionCaller) GetVertexAttribBinding(index int, pname Enum) Object
		 "gioui.org/internal/gl.asmGetVertexAttribBinding": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getVertexAttrib(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            gioSetObject((go._inst.exports.getsp() >>> 0)+0+8+8, r)
        },
         // (f *FunctionCaller) GetVertexAttribPointer(index int, pname Enum) uintptr
		 "gioui.org/internal/gl.asmGetVertexAttribPointer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.getVertexAttribOffset(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8+8, r)
        },
         // (f *FunctionCaller) InvalidateFramebuffer(target, attachment Enum)
		 "gioui.org/internal/gl.asmInvalidateFramebuffer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.invalidateFramebuffer(
				gioLoadInt64((sp)+0),
				[gioLoadInt64(sp+0+8)],
			);

            
        },
         // (f *FunctionCaller) IsEnabled(cap Enum) bool
		 "gioui.org/internal/gl.asmIsEnabled": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.isEnabled(
				gioLoadInt64((sp)+0),
			);

            gioSetInt64((go._inst.exports.getsp() >>> 0)+0+8, r)
        },
         // (f *FunctionCaller) LinkProgram(p Program)
		 "gioui.org/internal/gl.asmLinkProgram": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.linkProgram(
				gioLoadObject((sp)+0),
			);

            
        },
         // (f *FunctionCaller) PixelStorei(pname Enum, param int)
		 "gioui.org/internal/gl.asmPixelStorei": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.pixelStorei(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) RenderbufferStorage(target, internalformat Enum, width, height int)
		 "gioui.org/internal/gl.asmRenderbufferStorage": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.renderbufferStorage(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
			);

            
        },
         // (f *FunctionCaller) ReadPixels(x, y, width, height int, format, ty Enum, data []byte)
		 "gioui.org/internal/gl.asmReadPixels": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.readPixels(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8),
				gioLoadSlice((sp)+0+8+8+8+8+8+8),
			);

            
        },
         // (f *FunctionCaller) Scissor(x, y, width, height int32)
		 "gioui.org/internal/gl.asmScissor": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.scissor(
				gioLoadInt32((sp)+0),
				gioLoadInt32((sp)+0+8),
				gioLoadInt32((sp)+0+8+8),
				gioLoadInt32((sp)+0+8+8+8),
			);

            
        },
         // (f *FunctionCaller) ShaderSource(s Shader, src string)
		 "gioui.org/internal/gl.asmShaderSource": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.shaderSource(
				gioLoadObject((sp)+0),
				gioLoadString((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) TexImage2D(target Enum, level int, internalFormat Enum, width, height int, format, ty Enum)
		 "gioui.org/internal/gl.asmTexImage2D": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.texImage2D(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8),
				0,
				gioLoadInt64((sp)+0+8+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8+8),
				undefined,
			);

            
        },
         // (f *FunctionCaller) TexStorage2D(target Enum, levels int, internalFormat Enum, width, height int)
		 "gioui.org/internal/gl.asmTexStorage2D": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.texStorage2D(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8),
			);

            
        },
         // (f *FunctionCaller) TexSubImage2D(target Enum, level int, x, y, width, height int, format, ty Enum, data []byte)
		 "gioui.org/internal/gl.asmTexSubImage2D": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.texSubImage2D(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8+8+8),
				gioLoadSlice((sp)+0+8+8+8+8+8+8+8+8),
			);

            
        },
         // (f *FunctionCaller) TexParameteri(target, pname Enum, param int)
		 "gioui.org/internal/gl.asmTexParameteri": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.texParameteri(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) UniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint)
		 "gioui.org/internal/gl.asmUniformBlockBinding": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.uniformBlockBinding(
				gioLoadObject((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) Uniform1f(dst Uniform, v float32)
		 "gioui.org/internal/gl.asmUniform1f": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.uniform1f(
				gioLoadObject((sp)+0),
				gioLoadFloat64((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) Uniform1i(dst Uniform, v int)
		 "gioui.org/internal/gl.asmUniform1i": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.uniform1i(
				gioLoadObject((sp)+0),
				gioLoadInt64((sp)+0+8),
			);

            
        },
         // (f *FunctionCaller) Uniform2f(dst Uniform, v0, v1 float32)
		 "gioui.org/internal/gl.asmUniform2f": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.uniform2f(
				gioLoadObject((sp)+0),
				gioLoadFloat64((sp)+0+8),
				gioLoadFloat64((sp)+0+8+8),
			);

            
        },
         // (f *FunctionCaller) Uniform3f(dst Uniform, v0, v1, v2 float32)
		 "gioui.org/internal/gl.asmUniform3f": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.uniform3f(
				gioLoadObject((sp)+0),
				gioLoadFloat64((sp)+0+8),
				gioLoadFloat64((sp)+0+8+8),
				gioLoadFloat64((sp)+0+8+8+8),
			);

            
        },
         // (f *FunctionCaller) Uniform4f(dst Uniform, v0, v1, v2, v3 float32)
		 "gioui.org/internal/gl.asmUniform4f": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.uniform4f(
				gioLoadObject((sp)+0),
				gioLoadFloat64((sp)+0+8),
				gioLoadFloat64((sp)+0+8+8),
				gioLoadFloat64((sp)+0+8+8+8),
				gioLoadFloat64((sp)+0+8+8+8+8),
			);

            
        },
         // (f *FunctionCaller) UseProgram(p Program)
		 "gioui.org/internal/gl.asmUseProgram": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.useProgram(
				gioLoadObject((sp)+0),
			);

            
        },
         // (f *FunctionCaller) VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride, offset int)
		 "gioui.org/internal/gl.asmVertexAttribPointer": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.vertexAttribPointer(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadBool((sp)+0+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8),
				gioLoadInt64((sp)+0+8+8+8+8+8),
			);

            
        },
         // (f *FunctionCaller) Viewport(x, y, width, height int)
		 "gioui.org/internal/gl.asmViewport": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.viewport(
				gioLoadInt64((sp)+0),
				gioLoadInt64((sp)+0+8),
				gioLoadInt64((sp)+0+8+8),
				gioLoadInt64((sp)+0+8+8+8),
			);

            
        },
	})
})();