(() => {

    let textDecoder = new TextDecoder("utf-8");
    let textEncoder = new TextEncoder("utf-8");

    let buffer = {}

    let values = [undefined]
    let valuesPool = []

    let methods = []

    const TypeNull = 0 | 0x10
    const TypeByte = 1 | 0x10
    const TypeInt32 = 2 | 0x10
    const TypeInt64 = 3 | 0x10
    const TypeFloat32 = 4 | 0x10
    const TypeFloat64 = 5 | 0x10
    const TypeBoolean = 6 | 0x10
    const TypeString = 7 | 0x10
    const TypeSlice = 8 | 0x30
    const TypeSlice32 = 9 | 0x30
    const TypeValue = 10 | 0x40
    const TypeMethod = 11 | 0x40

    let types = {
        [TypeNull]: {
            Size: 8,
            Encoder: function (addr, v) {
            },
            Decoder: function (addr) {
            }
        },
        [TypeByte]: {
            Size: 8,
            Encoder: function (addr, v) {
                go.mem.setUint32(addr + 12, 0, true);
                go.mem.setUint32(addr + 8, v | 0, true);
            },
            Decoder: function (addr) {
                return go.mem.getUint8(addr + 8);
            },
        },
        [TypeInt32]: {
            Size: 8,
            Encoder: function (addr, v) {
                go.mem.setUint32(addr + 12, 0, true);
                go.mem.setUint32(addr + 8, v | 0, true);
            },
            Decoder: function (addr) {
                return go.mem.getUint32(addr + 8, true);
            },
        },
        [TypeInt64]: {
            Size: 8,
            Encoder: function (addr, v) {
                go.mem.setUint32(addr + 12, 0, true);
                go.mem.setUint32(addr + 8, v | 0, true);
            },
            Decoder: function (addr) {
                return go.mem.getUint32(addr + 8, true) + (go.mem.getUint32(addr + 12, true) * 4294967296);
            },
        },
        [TypeFloat32]: {
            Size: 8,
            Encoder: function (addr, v) {
                go.mem.setUint32(addr + 12, 0, true);
                go.mem.setFloat32(addr + 8, v, true);
            },
            Decoder: function (addr) {
                return go.mem.getFloat32(addr + 8, true);
            },
        },
        [TypeFloat64]: {
            Size: 8,
            Encoder: function (addr, v) {
                go.mem.setFloat64(addr + 8, v, true);
            },
            Decoder: function (addr) {
                return go.mem.getFloat64(addr + 8, true);
            },
        },
        [TypeBoolean]: {
            Size: 8,
            Encoder: function (addr, v) {
                go.mem.setUint32(addr + 12, 0, true);
                go.mem.setUint32(addr + 8, v | 0, true);
            },
            Decoder: function (addr) {
                return types[TypeByte].Decoder(addr) === 1;
            },
        },
        [TypeString]: {
            Size: 16,
            Encoder: function (addr, v) {
                let len = 0;
                if (textDecoder.encodeInto !== undefined) {
                    const r = textEncoder.encodeInto(v.toString(), buffer);
                    len = r.written;
                } else {
                    // Some browsers (Safari) doesn't support TextDecoder.encodeInto():
                    const r = textEncoder.encode(v);
                    buffer.set(r);
                    len = r.length;
                }
                types[TypeInt64].Encoder(addr, len);
            },
            Decoder: function (addr) {
                return textDecoder.decode(new DataView(go._inst.exports.mem.buffer, types[TypeInt32].Decoder(addr), types[TypeInt32].Decoder(addr + 8)));
            },
        },
        [TypeSlice]: {
            Size: 24,
            Encoder: function (addr, v) {
                // Output not supported
            },
            Decoder: function (addr) {
                const s = new Uint8Array(go._inst.exports.mem.buffer, types[TypeInt64].Decoder(addr), types[TypeInt64].Decoder(addr + 8))
                if (s.byteLength === 0) {
                    return null
                }
                return s
            },
        },
        [TypeSlice32]: {
            Size: 24,
            Encoder: function (addr, v) {
                // Output not supported
            },
            Decoder: function (addr) {
                const s = new Uint32Array(go._inst.exports.mem.buffer, types[TypeInt64].Decoder(addr), types[TypeInt64].Decoder(addr + 8))
                if (s.byteLength === 0) {
                    return null;
                }
                return s;
            },
        },
        [TypeValue]: {
            Size: 8,
            Encoder: function (addr, v) {
                let id = 0;
                if (v !== undefined && v !== null) {
                    id = valuesPool.pop();
                    if (id !== undefined) {
                        values[id] = v;
                    } else {
                        id = values.push(v) - 1;
                    }
                }
                types[TypeInt64].Encoder(addr, id);
            },
            Decoder: function (addr) {
                return values[types[TypeInt32].Decoder(addr)];
            },
        },
        [TypeMethod]: {
            Size: 8,
            Encoder: function (addr, v) {
                const id = methods.push(v);
                types[TypeInt64].Encoder(addr, id - 1);
            },
            Decoder: function (addr) {
                return methods[types[TypeByte].Decoder(addr)];
            },
        },
    }

    global.GlimpContext = (v) => {
        return values.push(v) - 1;
    }

    Object.assign(go.importObject.go, {
        // newMethod(function string, types []Type) Method
        "gioui.org/internal/glimpl.newMethod": (sp) => {
            const output = types[TypeByte].Decoder(sp);
            sp += types[TypeByte].Size;

            const func = types[TypeString].Decoder(sp);
            sp += types[TypeString].Size;

            const input = types[TypeSlice].Decoder(sp);
            sp += types[TypeSlice].Size;

            let m = {Name: func, Output: types[output], Input: []}
            if (input) {
                for (let i = 0; i < input.length; i++) {
                    m.Input[i] = types[input[i]];
                }
            }

            types[TypeMethod].Encoder(sp, m);
        },
        "gioui.org/internal/glimpl.get": (sp) => {
            const func = types[TypeValue].Decoder(sp);
            sp += types[TypeValue].Size;

            const method = types[TypeMethod].Decoder(sp);
            sp += types[TypeMethod].Size;

            const r = func[method.Name]
            if (r === undefined || r === null) {
                types[TypeInt64].Encoder(sp, 0);
            } else {
                method.Output.Encoder(sp, r);
            }
        },
        "gioui.org/internal/glimpl.call": (sp) => {
            const func = types[TypeValue].Decoder(sp);
            sp += types[TypeValue].Size;

            const method = types[TypeMethod].Decoder(sp);
            sp += types[TypeMethod].Size;

            let spArray = types[TypeInt64].Decoder(sp);
            sp += types[TypeSlice].Size;

            const args = new Array(method.Input.length);
            for (let i = 0; i < method.Input.length; i++) {
                args[i] = method.Input[i].Decoder(types[TypeInt64].Decoder(spArray - 8) - 8);
                spArray += 8;
            }

            const r = func[method.Name](...args);

            if (r === undefined || r === null) {
                types[TypeInt64].Encoder(sp, 0);
            } else {
                method.Output.Encoder(sp, r);
            }
        },
        "gioui.org/internal/glimpl.free": (sp) => {
            const ref = types[TypeInt64].Decoder(sp);
            valuesPool.push(ref);
        },
        "gioui.org/internal/glimpl.buffer": (sp) => {
            buffer = types[TypeSlice].Decoder(sp);
        },
    })

})();