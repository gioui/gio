package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/akavel/rsrc/binutil"
	"github.com/akavel/rsrc/coff"
	"golang.org/x/text/encoding/unicode"
)

func buildWindows(tmpDir string, bi *buildInfo) error {
	builder := &windowsBuilder{TempDir: tmpDir}
	builder.DestDir = *destPath
	if builder.DestDir == "" {
		builder.DestDir = bi.pkgPath
	}

	name := bi.name
	if *destPath != "" {
		if filepath.Ext(*destPath) != ".exe" {
			return fmt.Errorf("invalid output name %q, it must end with `.exe`", *destPath)
		}
		name = filepath.Base(*destPath)
	}
	name = strings.TrimSuffix(name, ".exe")
	sdk := bi.minsdk
	if sdk > 10 {
		return fmt.Errorf("invalid minsdk (%d) it's higher than Windows 10", sdk)
	}
	version := strconv.Itoa(bi.version)
	if bi.version > math.MaxUint16 {
		return fmt.Errorf("version (%d) is larger than the maximum (%d)", bi.version, math.MaxUint16)
	}

	for _, arch := range bi.archs {
		builder.Coff = coff.NewRSRC()
		builder.Coff.Arch(arch)

		if err := builder.embedIcon(bi.iconPath); err != nil {
			return err
		}

		if err := builder.embedManifest(windowsManifest{
			Version:        "1.0.0." + version,
			WindowsVersion: sdk,
			Name:           name,
		}); err != nil {
			return fmt.Errorf("can't create manifest: %v", err)
		}

		if err := builder.embedInfo(windowsResources{
			Version:      [2]uint32{uint32(1) << 16, uint32(bi.version)},
			VersionHuman: "1.0.0." + version,
			Name:         name,
			Language:     0x0400, // Process Default Language: https://docs.microsoft.com/en-us/previous-versions/ms957130(v=msdn.10)
		}); err != nil {
			return fmt.Errorf("can't create info: %v", err)
		}

		if err := builder.buildResource(bi, name, arch); err != nil {
			return fmt.Errorf("can't build the resources: %v", err)
		}

		if err := builder.buildProgram(bi, name, arch); err != nil {
			return err
		}
	}

	return nil
}

type (
	windowsResources struct {
		Version      [2]uint32
		VersionHuman string
		Language     uint16
		Name         string
	}
	windowsManifest struct {
		Version        string
		WindowsVersion int
		Name           string
	}
	windowsBuilder struct {
		TempDir string
		DestDir string
		Coff    *coff.Coff
	}
)

const (
	// https://docs.microsoft.com/en-us/windows/win32/menurc/resource-types
	windowsResourceIcon      = 3
	windowsResourceIconGroup = windowsResourceIcon + 11
	windowsResourceManifest  = 24
	windowsResourceVersion   = 16
)

type bufferCoff struct {
	bytes.Buffer
}

func (b *bufferCoff) Size() int64 {
	return int64(b.Len())
}

func (b *windowsBuilder) embedIcon(path string) (err error) {
	iconFile, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("can't read the icon located at %s: %v", path, err)
	}
	defer iconFile.Close()

	iconImage, err := png.Decode(iconFile)
	if err != nil {
		return fmt.Errorf("can't decode the PNG file (%s): %v", path, err)
	}

	sizes := []int{16, 32, 48, 64, 128, 256}
	var iconHeader bufferCoff

	// GRPICONDIR structure.
	if err := binary.Write(&iconHeader, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))}); err != nil {
		return err
	}

	for _, size := range sizes {
		var iconBuffer bufferCoff

		if err := png.Encode(&iconBuffer, resizeIcon(iconVariant{size: size, fill: false}, iconImage)); err != nil {
			return fmt.Errorf("can't encode image: %v", err)
		}

		b.Coff.AddResource(windowsResourceIcon, uint16(size), &iconBuffer)

		if err := binary.Write(&iconHeader, binary.LittleEndian, struct {
			Size     [2]uint8
			Color    [2]uint8
			Planes   uint16
			BitCount uint16
			Length   uint32
			Id       uint16
		}{
			Size:     [2]uint8{uint8(size % 256), uint8(size % 256)}, // "0" means 256px.
			Planes:   1,
			BitCount: 32,
			Length:   uint32(iconBuffer.Len()),
			Id:       uint16(size),
		}); err != nil {
			return err
		}
	}

	b.Coff.AddResource(windowsResourceIconGroup, 1, &iconHeader)

	return nil
}

func (b *windowsBuilder) buildResource(buildInfo *buildInfo, name string, arch string) error {
	out, err := os.Create(filepath.Join(buildInfo.pkgPath, name+"_windows_"+arch+".syso"))
	if err != nil {
		return err
	}
	defer out.Close()
	b.Coff.Freeze()

	// See https://github.com/akavel/rsrc/internal/write.go#L13.
	w := binutil.Writer{W: out}
	binutil.Walk(b.Coff, func(v reflect.Value, path string) error {
		if binutil.Plain(v.Kind()) {
			w.WriteLE(v.Interface())
			return nil
		}
		vv, ok := v.Interface().(binutil.SizedReader)
		if ok {
			w.WriteFromSized(vv)
			return binutil.WALK_SKIP
		}
		return nil
	})

	if w.Err != nil {
		return fmt.Errorf("error writing output file: %s", w.Err)
	}

	return nil
}

func (b *windowsBuilder) buildProgram(buildInfo *buildInfo, name string, arch string) error {
	dest := b.DestDir
	if len(buildInfo.archs) > 1 {
		dest = filepath.Join(filepath.Dir(b.DestDir), name+"_"+arch+".exe")
	}

	cmd := exec.Command(
		"go",
		"build",
		"-ldflags=-H=windowsgui "+buildInfo.ldflags,
		"-tags="+buildInfo.tags,
		"-o", dest,
		buildInfo.pkgPath,
	)
	cmd.Env = append(
		os.Environ(),
		"GOOS=windows",
		"GOARCH="+arch,
	)
	_, err := runCmd(cmd)
	return err
}

func (b *windowsBuilder) embedManifest(v windowsManifest) error {
	t, err := template.New("manifest").Parse(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly manifestVersion="1.0" xmlns="urn:schemas-microsoft-com:asm.v1" xmlns:asmv3="urn:schemas-microsoft-com:asm.v3">
    <assemblyIdentity type="win32" name="{{.Name}}" version="{{.Version}}" />
    <description>{{.Name}}</description>
    <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
        <application>
            {{if (le .WindowsVersion 10)}}<supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
{{end}}
            {{if (le .WindowsVersion 9)}}<supportedOS Id="{1f676c76-80e1-4239-95bb-83d0f6d0da78}"/>
{{end}}
            {{if (le .WindowsVersion 8)}}<supportedOS Id="{4a2f28e3-53b9-4441-ba9c-d69d4a4a6e38}"/>
{{end}}
            {{if (le .WindowsVersion 7)}}<supportedOS Id="{35138b9a-5d96-4fbd-8e2d-a2440225f93a}"/>
{{end}}
            {{if (le .WindowsVersion 6)}}<supportedOS Id="{e2011457-1546-43c5-a5fe-008deee3d3f0}"/>
{{end}}
        </application>
    </compatibility>
    <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
        <security>
            <requestedPrivileges>
                <requestedExecutionLevel level="asInvoker" uiAccess="false" />
            </requestedPrivileges>
        </security>
    </trustInfo>
	<asmv3:application>
		<asmv3:windowsSettings>
			<dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true</dpiAware>
		</asmv3:windowsSettings>
	</asmv3:application>
</assembly>`)
	if err != nil {
		return err
	}

	var manifest bufferCoff
	if err := t.Execute(&manifest, v); err != nil {
		return err
	}

	b.Coff.AddResource(windowsResourceManifest, 1, &manifest)

	return nil
}

func (b *windowsBuilder) embedInfo(v windowsResources) error {
	page := uint16(1)

	// https://docs.microsoft.com/pt-br/windows/win32/menurc/vs-versioninfo
	t := newValue(valueBinary, "VS_VERSION_INFO", []io.WriterTo{
		// https://docs.microsoft.com/pt-br/windows/win32/api/VerRsrc/ns-verrsrc-vs_fixedfileinfo
		windowsInfoValueFixed{
			Signature:      0xFEEF04BD,
			StructVersion:  0x00010000,
			FileVersion:    v.Version,
			ProductVersion: v.Version,
			FileFlagMask:   0x3F,
			FileFlags:      0,
			FileOS:         0x40004,
			FileType:       0x1,
			FileSubType:    0,
		},
		// https://docs.microsoft.com/pt-br/windows/win32/menurc/stringfileinfo
		newValue(valueText, "StringFileInfo", []io.WriterTo{
			// https://docs.microsoft.com/pt-br/windows/win32/menurc/stringtable
			newValue(valueText, fmt.Sprintf("%04X%04X", v.Language, page), []io.WriterTo{
				// https://docs.microsoft.com/pt-br/windows/win32/menurc/string-str
				newValue(valueText, "ProductVersion", v.VersionHuman),
				newValue(valueText, "FileVersion", v.VersionHuman),
				newValue(valueText, "FileDescription", v.Name),
				newValue(valueText, "ProductName", v.Name),
				// TODO include more data: gogio must have some way to provide such information (like Company Name, Copyright...)
			}),
		}),
		// https://docs.microsoft.com/pt-br/windows/win32/menurc/varfileinfo
		newValue(valueBinary, "VarFileInfo", []io.WriterTo{
			// https://docs.microsoft.com/pt-br/windows/win32/menurc/var-str
			newValue(valueBinary, "Translation", uint32(page)<<16|uint32(v.Language)),
		}),
	})

	// For some reason the ValueLength of the VS_VERSIONINFO must be the byte-length of `windowsInfoValueFixed`:
	t.ValueLength = 52

	var verrsrc bufferCoff
	if _, err := t.WriteTo(&verrsrc); err != nil {
		return err
	}

	b.Coff.AddResource(windowsResourceVersion, 1, &verrsrc)

	return nil
}

type windowsInfoValueFixed struct {
	Signature      uint32
	StructVersion  uint32
	FileVersion    [2]uint32
	ProductVersion [2]uint32
	FileFlagMask   uint32
	FileFlags      uint32
	FileOS         uint32
	FileType       uint32
	FileSubType    uint32
	FileDate       [2]uint32
}

func (v windowsInfoValueFixed) WriteTo(w io.Writer) (_ int64, err error) {
	return 0, binary.Write(w, binary.LittleEndian, v)
}

type windowsInfoValue struct {
	Length      uint16
	ValueLength uint16
	Type        uint16
	Key         []byte
	Value       []byte
}

func (v windowsInfoValue) WriteTo(w io.Writer) (_ int64, err error) {
	// binary.Write doesn't support []byte inside struct.
	if err = binary.Write(w, binary.LittleEndian, [3]uint16{v.Length, v.ValueLength, v.Type}); err != nil {
		return 0, err
	}
	if _, err = w.Write(v.Key); err != nil {
		return 0, err
	}
	if _, err = w.Write(v.Value); err != nil {
		return 0, err
	}
	return 0, nil
}

const (
	valueBinary uint16 = 0
	valueText   uint16 = 1
)

func newValue(valueType uint16, key string, input interface{}) windowsInfoValue {
	v := windowsInfoValue{
		Type:   valueType,
		Length: 6,
	}

	padding := func(in []byte) []byte {
		if l := uint16(len(in)) + v.Length; l%4 != 0 {
			return append(in, make([]byte, 4-l%4)...)
		}
		return in
	}

	v.Key = padding(utf16Encode(key))
	v.Length += uint16(len(v.Key))

	switch in := input.(type) {
	case string:
		v.Value = padding(utf16Encode(in))
		v.ValueLength = uint16(len(v.Value) / 2)
	case []io.WriterTo:
		var buff bytes.Buffer
		for k := range in {
			if _, err := in[k].WriteTo(&buff); err != nil {
				panic(err)
			}
		}
		v.Value = buff.Bytes()
	default:
		var buff bytes.Buffer
		if err := binary.Write(&buff, binary.LittleEndian, in); err != nil {
			panic(err)
		}
		v.ValueLength = uint16(buff.Len())
		v.Value = buff.Bytes()
	}

	v.Length += uint16(len(v.Value))

	return v
}

// utf16Encode encodes the string to UTF16 with null-termination.
func utf16Encode(s string) []byte {
	b, err := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder().Bytes([]byte(s))
	if err != nil {
		panic(err)
	}
	return append(b, 0x00, 0x00) // null-termination.
}
