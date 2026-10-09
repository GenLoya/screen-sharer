//go:build windows

package main

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

// DXGI is the API ffmpeg's ddagrab uses, so enumerating its outputs here keeps
// display numbers and bounds consistent with what ddagrab captures.

var procCreateDXGIFactory1 = syscall.NewLazyDLL("dxgi.dll").NewProc("CreateDXGIFactory1")

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var iidIDXGIFactory1 = guid{0x770aae78, 0xf26f, 0x4dba, [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}

// COM vtable indexes.
const (
	methodRelease            = 2
	methodFactoryEnumAdapter = 7 // IDXGIFactory::EnumAdapters
	methodAdapterEnumOutputs = 7 // IDXGIAdapter::EnumOutputs
	methodOutputGetDesc      = 7 // IDXGIOutput::GetDesc
)

type dxgiOutputDesc struct {
	DeviceName         [32]uint16
	DesktopCoordinates struct{ Left, Top, Right, Bottom int32 }
	AttachedToDesktop  int32
	Rotation           uint32
	Monitor            uintptr
}

type dxgiOutput struct {
	name   string
	bounds image.Rectangle // desktop coordinates in physical pixels
}

func comCall(obj unsafe.Pointer, method int, args ...uintptr) uintptr {
	vtbl := *(**[16]uintptr)(obj)
	r, _, _ := syscall.SyscallN(vtbl[method], append([]uintptr{uintptr(obj)}, args...)...)
	return r
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

// dxgiOutputs lists the outputs of the default adapter, in ddagrab's output_idx order.
func dxgiOutputs() ([]dxgiOutput, error) {
	var factory unsafe.Pointer
	if hr, _, _ := procCreateDXGIFactory1.Call(
		uintptr(unsafe.Pointer(&iidIDXGIFactory1)), uintptr(unsafe.Pointer(&factory)),
	); failed(hr) {
		return nil, fmt.Errorf("CreateDXGIFactory1: 0x%08x", uint32(hr))
	}
	defer comCall(factory, methodRelease)

	var adapter unsafe.Pointer
	if hr := comCall(factory, methodFactoryEnumAdapter, 0, uintptr(unsafe.Pointer(&adapter))); failed(hr) {
		return nil, fmt.Errorf("EnumAdapters: 0x%08x", uint32(hr))
	}
	defer comCall(adapter, methodRelease)

	var outputs []dxgiOutput
	for i := 0; ; i++ {
		var output unsafe.Pointer
		if hr := comCall(adapter, methodAdapterEnumOutputs, uintptr(i), uintptr(unsafe.Pointer(&output))); failed(hr) {
			break // DXGI_ERROR_NOT_FOUND: no more outputs
		}
		var desc dxgiOutputDesc
		hr := comCall(output, methodOutputGetDesc, uintptr(unsafe.Pointer(&desc)))
		comCall(output, methodRelease)
		if failed(hr) {
			return nil, fmt.Errorf("IDXGIOutput::GetDesc: 0x%08x", uint32(hr))
		}
		r := desc.DesktopCoordinates
		outputs = append(outputs, dxgiOutput{
			name:   syscall.UTF16ToString(desc.DeviceName[:]),
			bounds: image.Rect(int(r.Left), int(r.Top), int(r.Right), int(r.Bottom)),
		})
	}
	return outputs, nil
}
