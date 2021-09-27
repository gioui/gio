package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

func translateArgType(s string) (f string, offset string) {
	switch s {
	case "bool":
		return "gioLoadBool", "8"
	case "string":
		return "gioLoadString", "16"
	case "int", "int64", "uint", "Attrib", "Enum", "uintptr", "unit64":
		return "gioLoadInt64", "8"
	case "int32", "uint32":
		return "gioLoadInt32", "8"
	case "[]byte":
		return "gioLoadSlice", "24"
	case "float", "float64", "float32":
		return "gioLoadFloat64", "8"
	default:
		return "gioLoadObject", "8"
	}
}

func translateResultType(s string) (f string, offset string) {
	switch s {
	case "int", "int64", "uint", "Attrib", "Enum", "uintptr", "bool":
		return "gioSetInt64", "8"
	case "[4]float32", "[4]int":
		return "gioSetArray4", "32"
	default:
		return "gioSetObject", "8"
	}
}

func main() {

	data, err := ioutil.ReadFile("Z:\\gio\\gio-3\\internal\\gl\\gl_syscall_js.go")
	if err != nil {
		panic(err)
	}

	js, err := os.Create("Z:\\gio\\gio-3\\internal\\gl\\gl_unsafe_js.js")
	if err != nil {
		panic(err)
	}

	golang, err := os.Create("Z:\\gio\\gio-3\\internal\\gl\\gl_unsafe_js.go")
	if err != nil {
		panic(err)
	}

	asm, err := os.Create("Z:\\gio\\gio-3\\internal\\gl\\gl_unsafe_js.s")
	if err != nil {
		panic(err)
	}

	findFunctions := regexp.MustCompile(`\(f \*FunctionCaller\) (\w+)\((.*)\) (.*?){`).FindAllSubmatch(data, -1)
	findCalls := regexp.MustCompile(`f.Ctx.Call\((.*)\)`).FindAllSubmatch(data, -1)

	typeRemover := regexp.MustCompile(`(.*)\((\w+)\)`)

	writeHeader(js)
	writeGoHeader(golang)

	asm.WriteString(`// SPDX-License-Identifier: Unlicense OR MIT

#include "textflag.h"

`)

	for i, v := range findFunctions {
		if strings.ToUpper(string(v[1][0])) != string(v[1][0]) {
			continue
		}
		if len(findCalls[i][1]) == 0 {
			continue
		}
		if strings.Contains(string(v[0]), "panic") {
			continue
		}

		for _, v := range v {
			fmt.Println(string(v))
		}
		for _, v := range findCalls[i] {
			fmt.Println(string(v))
		}

		argsRaw := strings.Split(string(v[2]), ", ")

		args := make([][2]string, len(argsRaw))

		for i, a := range argsRaw {
			aa := strings.Split(a, " ")
			args[i][0] = strings.TrimSpace(aa[0])

			if len(aa) > 1 {
				args[i][1] = aa[1]

				for it := 0; it < i; it++ {
					if args[it][1] == "" {
						args[it][1] = strings.TrimSpace(aa[1])
					}
				}
			}
		}

		var argsJS string
		var offset = "0"

		fmt.Println(args, len(args))

		jsArgs := strings.Split(string(findCalls[i][1]), ",")

		asmArgs := make([]string, 0, len(jsArgs))
		goArgsNames := make([]string, 0, len(jsArgs))
		for _, a := range jsArgs {
			t := ""
			for _, aa := range args {
				if cast := typeRemover.FindAllStringSubmatch(a, -1); len(cast) >= 1 {
					a = cast[0][2]
				}
				if a == "ba" {
					a = "data"
				}
				a = strings.Trim(strings.TrimSpace(a), `)`)
				fmt.Println(a, aa[0])
				if aa[0] == a {
					t = aa[1]
					break
				}
			}
			isArray := false
			if a == "f.int32Buf" && string(v[1]) == "InvalidateFramebuffer" {
				//specal case
				t = "Enum"
				a = "attachment"
				isArray = true
			}
			if t == "" {
				switch a {
				case "nil":
					argsJS += fmt.Sprintf("\t\t\t\tundefined,\n")
				case "0":
					argsJS += fmt.Sprintf("\t\t\t\t0,\n")
				}
				continue
			}
			f, o := translateArgType(t)

			if t == "float32" {
				t = "float64"
			}
			asmArgs = append(asmArgs, a+" "+t)

			if t == "float64" {
				a = "float64(" + a + ")"
			}
			goArgsNames = append(goArgsNames, a)

			if isArray {
				argsJS += fmt.Sprintf("\t\t\t\t[%s(sp+%s)],\n", f, offset)
			} else {
				argsJS += fmt.Sprintf("\t\t\t\t%s((sp)+%s),\n", f, offset)
			}
			offset += "+" + o
		}

		resultJS := ""
		resultGo := ""
		resultGoType := ""
		if len(v) > 3 {
			result := strings.TrimSpace(string(v[3]))
			if len(result) > 0 {
				f, _ := translateResultType(result)

				resultGoType = result
				resultJS = fmt.Sprintf("%s((go._inst.exports.getsp() >>> 0)+%s, r)", f, offset)
				resultGo = "return "
			}
			if strings.Contains(string(v[1]), "Delete") {
				resultJS = fmt.Sprintf("gioDeleteObject(sp)")
			}
		}

		argsJS = strings.Trim(argsJS, ",")

		call := strings.Split(string(findCalls[i][1]), `, `)

		fmt.Fprintf(js, `
         // %s
		 "gioui.org/internal/gl.%s": (sp) => {
            sp = (sp >>> 0);

            let r = webgl.%s(
%s			);

            %s
        },`, strings.TrimSpace(strings.TrimRight(string(v[0]), `{`)), "asm"+string(v[1]), strings.Replace(strings.Replace(call[0], `"`, "", -1), `)`, ``, -1), argsJS, resultJS)

		fmt.Fprintf(golang, `
//go:noescape
func %s(%s) %s

func %s
	%s%s(%s)
}
`, "asm"+string(v[1]), strings.Join(asmArgs, ", "), resultGoType, v[0], resultGo, "asm"+string(v[1]), strings.Join(goArgsNames, ", "))

		fmt.Fprintf(asm, `TEXT ·%s(SB), NOSPLIT, $0
  CallImport
  RET

`, "asm"+string(v[1]))
	}
	writeEnd(js)
}

func writeHeader(f io.StringWriter) {
	f.WriteString(`(() => {

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

    Object.assign(go.importObject.go, {`)
}

func writeEnd(file io.StringWriter) {
	file.WriteString(`
	})
})();`)
}

func writeGoHeader(file io.StringWriter) {
	file.WriteString(`//+build unsafe
// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"syscall/js"
)

type FunctionCaller struct {}

func NewFunctionCaller(ctx Context) *FunctionCaller {
	js.Global().Call("setUnsafeGL", js.Value(ctx))
	return &FunctionCaller{}
}
`)

}
