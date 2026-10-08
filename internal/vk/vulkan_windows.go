// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package vk

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"github.com/christerso/vulkan-go/vulkan"
	"github.com/ebitengine/purego"
)

// The Windows implementation mirrors the Linux cgo implementation in
// vulkan.go, but loads the Vulkan loader (vulkan-1.dll) and binds its entry
// points with purego instead of dlopen. Structs, enums and raw entry points
// come from the generated, cgo-free github.com/christerso/vulkan-go package.
//
// Dispatchable Vulkan handles (instance, device, queue, physical device,
// command buffer) are pointer sized on 64 bit platforms; the C implementation
// treats them as pointers, so this file aliases them to unsafe.Pointer to keep
// nil checks working the same way. Non-dispatchable handles are 64 bit on all
// platforms and alias the vulkan-go definitions directly.

type (
	AttachmentLoadOp      = vulkan.VkAttachmentLoadOp
	AccessFlags           = vulkan.VkAccessFlags
	BlendFactor           = vulkan.VkBlendFactor
	Buffer                = vulkan.VkBuffer
	BufferImageCopy       = vulkan.VkBufferImageCopy
	BufferMemoryBarrier   = vulkan.VkBufferMemoryBarrier
	BufferUsageFlags      = vulkan.VkBufferUsageFlags
	CommandPool           = vulkan.VkCommandPool
	CommandBuffer         = unsafe.Pointer
	DependencyFlags       = vulkan.VkDependencyFlags
	DescriptorPool        = vulkan.VkDescriptorPool
	DescriptorPoolSize    = vulkan.VkDescriptorPoolSize
	DescriptorSet         = vulkan.VkDescriptorSet
	DescriptorSetLayout   = vulkan.VkDescriptorSetLayout
	DescriptorType        = vulkan.VkDescriptorType
	Device                = unsafe.Pointer
	DeviceMemory          = vulkan.VkDeviceMemory
	DeviceSize            = vulkan.VkDeviceSize
	Fence                 = vulkan.VkFence
	Queue                 = unsafe.Pointer
	IndexType             = vulkan.VkIndexType
	Image                 = vulkan.VkImage
	ImageBlit             = vulkan.VkImageBlit
	ImageCopy             = vulkan.VkImageCopy
	ImageLayout           = vulkan.VkImageLayout
	ImageMemoryBarrier    = vulkan.VkImageMemoryBarrier
	ImageUsageFlags       = vulkan.VkImageUsageFlags
	ImageView             = vulkan.VkImageView
	Instance              = unsafe.Pointer
	Filter                = vulkan.VkFilter
	Format                = vulkan.VkFormat
	FormatFeatureFlags    = vulkan.VkFormatFeatureFlags
	Framebuffer           = vulkan.VkFramebuffer
	MemoryBarrier         = vulkan.VkMemoryBarrier
	MemoryPropertyFlags   = vulkan.VkMemoryPropertyFlags
	Pipeline              = vulkan.VkPipeline
	PipelineBindPoint     = vulkan.VkPipelineBindPoint
	PipelineLayout        = vulkan.VkPipelineLayout
	PipelineStageFlags    = vulkan.VkPipelineStageFlags
	PhysicalDevice        = unsafe.Pointer
	PrimitiveTopology     = vulkan.VkPrimitiveTopology
	PushConstantRange     = vulkan.VkPushConstantRange
	QueueFamilyProperties = vulkan.VkQueueFamilyProperties
	QueueFlags            = vulkan.VkQueueFlags
	RenderPass            = vulkan.VkRenderPass
	Sampler               = vulkan.VkSampler
	SamplerMipmapMode     = vulkan.VkSamplerMipmapMode
	Semaphore             = vulkan.VkSemaphore
	ShaderModule          = vulkan.VkShaderModule
	ShaderStageFlags      = vulkan.VkShaderStageFlags
	SubpassDependency     = vulkan.VkSubpassDependency
	Viewport              = vulkan.VkViewport
	WriteDescriptorSet    = vulkan.VkWriteDescriptorSet

	Surface             = vulkan.VkSurfaceKHR
	SurfaceCapabilities = vulkan.VkSurfaceCapabilitiesKHR

	Swapchain = vulkan.VkSwapchainKHR
)

type VertexInputBindingDescription struct {
	Binding int
	Stride  int
}

type VertexInputAttributeDescription struct {
	Location int
	Binding  int
	Format   Format
	Offset   int
}

type DescriptorSetLayoutBinding struct {
	Binding        int
	DescriptorType DescriptorType
	StageFlags     ShaderStageFlags
}

type Error vulkan.VkResult

const (
	FORMAT_R8G8B8A8_UNORM      Format = vulkan.VK_FORMAT_R8G8B8A8_UNORM
	FORMAT_B8G8R8A8_SRGB       Format = vulkan.VK_FORMAT_B8G8R8A8_SRGB
	FORMAT_R8G8B8A8_SRGB       Format = vulkan.VK_FORMAT_R8G8B8A8_SRGB
	FORMAT_R16_SFLOAT          Format = vulkan.VK_FORMAT_R16_SFLOAT
	FORMAT_R32_SFLOAT          Format = vulkan.VK_FORMAT_R32_SFLOAT
	FORMAT_R32G32_SFLOAT       Format = vulkan.VK_FORMAT_R32G32_SFLOAT
	FORMAT_R32G32B32_SFLOAT    Format = vulkan.VK_FORMAT_R32G32B32_SFLOAT
	FORMAT_R32G32B32A32_SFLOAT Format = vulkan.VK_FORMAT_R32G32B32A32_SFLOAT

	FORMAT_FEATURE_COLOR_ATTACHMENT_BIT            FormatFeatureFlags = vulkan.VK_FORMAT_FEATURE_COLOR_ATTACHMENT_BIT
	FORMAT_FEATURE_COLOR_ATTACHMENT_BLEND_BIT      FormatFeatureFlags = vulkan.VK_FORMAT_FEATURE_COLOR_ATTACHMENT_BLEND_BIT
	FORMAT_FEATURE_SAMPLED_IMAGE_BIT               FormatFeatureFlags = vulkan.VK_FORMAT_FEATURE_SAMPLED_IMAGE_BIT
	FORMAT_FEATURE_SAMPLED_IMAGE_FILTER_LINEAR_BIT FormatFeatureFlags = vulkan.VK_FORMAT_FEATURE_SAMPLED_IMAGE_FILTER_LINEAR_BIT

	IMAGE_USAGE_SAMPLED_BIT          ImageUsageFlags = vulkan.VK_IMAGE_USAGE_SAMPLED_BIT
	IMAGE_USAGE_COLOR_ATTACHMENT_BIT ImageUsageFlags = vulkan.VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT
	IMAGE_USAGE_STORAGE_BIT          ImageUsageFlags = vulkan.VK_IMAGE_USAGE_STORAGE_BIT
	IMAGE_USAGE_TRANSFER_DST_BIT     ImageUsageFlags = vulkan.VK_IMAGE_USAGE_TRANSFER_DST_BIT
	IMAGE_USAGE_TRANSFER_SRC_BIT     ImageUsageFlags = vulkan.VK_IMAGE_USAGE_TRANSFER_SRC_BIT

	FILTER_NEAREST Filter = vulkan.VK_FILTER_NEAREST
	FILTER_LINEAR  Filter = vulkan.VK_FILTER_LINEAR

	ATTACHMENT_LOAD_OP_CLEAR     AttachmentLoadOp = vulkan.VK_ATTACHMENT_LOAD_OP_CLEAR
	ATTACHMENT_LOAD_OP_DONT_CARE AttachmentLoadOp = vulkan.VK_ATTACHMENT_LOAD_OP_DONT_CARE
	ATTACHMENT_LOAD_OP_LOAD      AttachmentLoadOp = vulkan.VK_ATTACHMENT_LOAD_OP_LOAD

	IMAGE_LAYOUT_UNDEFINED                ImageLayout = vulkan.VK_IMAGE_LAYOUT_UNDEFINED
	IMAGE_LAYOUT_PRESENT_SRC_KHR          ImageLayout = vulkan.VK_IMAGE_LAYOUT_PRESENT_SRC_KHR
	IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL ImageLayout = vulkan.VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL
	IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL ImageLayout = vulkan.VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL
	IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL     ImageLayout = vulkan.VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL
	IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL     ImageLayout = vulkan.VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL
	IMAGE_LAYOUT_GENERAL                  ImageLayout = vulkan.VK_IMAGE_LAYOUT_GENERAL

	BUFFER_USAGE_TRANSFER_DST_BIT   BufferUsageFlags = vulkan.VK_BUFFER_USAGE_TRANSFER_DST_BIT
	BUFFER_USAGE_TRANSFER_SRC_BIT   BufferUsageFlags = vulkan.VK_BUFFER_USAGE_TRANSFER_SRC_BIT
	BUFFER_USAGE_UNIFORM_BUFFER_BIT BufferUsageFlags = vulkan.VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT
	BUFFER_USAGE_STORAGE_BUFFER_BIT BufferUsageFlags = vulkan.VK_BUFFER_USAGE_STORAGE_BUFFER_BIT
	BUFFER_USAGE_INDEX_BUFFER_BIT   BufferUsageFlags = vulkan.VK_BUFFER_USAGE_INDEX_BUFFER_BIT
	BUFFER_USAGE_VERTEX_BUFFER_BIT  BufferUsageFlags = vulkan.VK_BUFFER_USAGE_VERTEX_BUFFER_BIT

	ERROR_OUT_OF_DATE_KHR  = Error(vulkan.VK_ERROR_OUT_OF_DATE_KHR)
	ERROR_SURFACE_LOST_KHR = Error(vulkan.VK_ERROR_SURFACE_LOST_KHR)
	ERROR_DEVICE_LOST      = Error(vulkan.VK_ERROR_DEVICE_LOST)
	SUBOPTIMAL_KHR         = Error(vulkan.VK_SUBOPTIMAL_KHR)

	FENCE_CREATE_SIGNALED_BIT = 0x00000001

	BLEND_FACTOR_ZERO                BlendFactor = vulkan.VK_BLEND_FACTOR_ZERO
	BLEND_FACTOR_ONE                 BlendFactor = vulkan.VK_BLEND_FACTOR_ONE
	BLEND_FACTOR_ONE_MINUS_SRC_ALPHA BlendFactor = vulkan.VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA
	BLEND_FACTOR_DST_COLOR           BlendFactor = vulkan.VK_BLEND_FACTOR_DST_COLOR

	PRIMITIVE_TOPOLOGY_TRIANGLE_LIST  PrimitiveTopology = vulkan.VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST
	PRIMITIVE_TOPOLOGY_TRIANGLE_STRIP PrimitiveTopology = vulkan.VK_PRIMITIVE_TOPOLOGY_TRIANGLE_STRIP

	SHADER_STAGE_VERTEX_BIT   ShaderStageFlags = vulkan.VK_SHADER_STAGE_VERTEX_BIT
	SHADER_STAGE_FRAGMENT_BIT ShaderStageFlags = vulkan.VK_SHADER_STAGE_FRAGMENT_BIT
	SHADER_STAGE_COMPUTE_BIT  ShaderStageFlags = vulkan.VK_SHADER_STAGE_COMPUTE_BIT

	DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER DescriptorType = vulkan.VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER
	DESCRIPTOR_TYPE_UNIFORM_BUFFER         DescriptorType = vulkan.VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER
	DESCRIPTOR_TYPE_STORAGE_BUFFER         DescriptorType = vulkan.VK_DESCRIPTOR_TYPE_STORAGE_BUFFER
	DESCRIPTOR_TYPE_STORAGE_IMAGE          DescriptorType = vulkan.VK_DESCRIPTOR_TYPE_STORAGE_IMAGE

	MEMORY_PROPERTY_DEVICE_LOCAL_BIT  MemoryPropertyFlags = vulkan.VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT
	MEMORY_PROPERTY_HOST_VISIBLE_BIT  MemoryPropertyFlags = vulkan.VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT
	MEMORY_PROPERTY_HOST_COHERENT_BIT MemoryPropertyFlags = vulkan.VK_MEMORY_PROPERTY_HOST_COHERENT_BIT

	DEPENDENCY_BY_REGION_BIT DependencyFlags = vulkan.VK_DEPENDENCY_BY_REGION_BIT

	PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT
	PIPELINE_STAGE_TRANSFER_BIT                PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_TRANSFER_BIT
	PIPELINE_STAGE_FRAGMENT_SHADER_BIT         PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT
	PIPELINE_STAGE_COMPUTE_SHADER_BIT          PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT
	PIPELINE_STAGE_TOP_OF_PIPE_BIT             PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT
	PIPELINE_STAGE_HOST_BIT                    PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_HOST_BIT
	PIPELINE_STAGE_VERTEX_INPUT_BIT            PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_VERTEX_INPUT_BIT
	PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT          PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT

	ACCESS_MEMORY_READ_BIT            AccessFlags = vulkan.VK_ACCESS_MEMORY_READ_BIT
	ACCESS_MEMORY_WRITE_BIT           AccessFlags = vulkan.VK_ACCESS_MEMORY_WRITE_BIT
	ACCESS_TRANSFER_READ_BIT          AccessFlags = vulkan.VK_ACCESS_TRANSFER_READ_BIT
	ACCESS_TRANSFER_WRITE_BIT         AccessFlags = vulkan.VK_ACCESS_TRANSFER_WRITE_BIT
	ACCESS_SHADER_READ_BIT            AccessFlags = vulkan.VK_ACCESS_SHADER_READ_BIT
	ACCESS_SHADER_WRITE_BIT           AccessFlags = vulkan.VK_ACCESS_SHADER_WRITE_BIT
	ACCESS_COLOR_ATTACHMENT_READ_BIT  AccessFlags = vulkan.VK_ACCESS_COLOR_ATTACHMENT_READ_BIT
	ACCESS_COLOR_ATTACHMENT_WRITE_BIT AccessFlags = vulkan.VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT
	ACCESS_HOST_READ_BIT              AccessFlags = vulkan.VK_ACCESS_HOST_READ_BIT
	ACCESS_HOST_WRITE_BIT             AccessFlags = vulkan.VK_ACCESS_HOST_WRITE_BIT
	ACCESS_VERTEX_ATTRIBUTE_READ_BIT  AccessFlags = vulkan.VK_ACCESS_VERTEX_ATTRIBUTE_READ_BIT
	ACCESS_INDEX_READ_BIT             AccessFlags = vulkan.VK_ACCESS_INDEX_READ_BIT

	PIPELINE_BIND_POINT_COMPUTE  PipelineBindPoint = vulkan.VK_PIPELINE_BIND_POINT_COMPUTE
	PIPELINE_BIND_POINT_GRAPHICS PipelineBindPoint = vulkan.VK_PIPELINE_BIND_POINT_GRAPHICS

	INDEX_TYPE_UINT16 IndexType = vulkan.VK_INDEX_TYPE_UINT16
	INDEX_TYPE_UINT32 IndexType = vulkan.VK_INDEX_TYPE_UINT32

	QUEUE_GRAPHICS_BIT QueueFlags = vulkan.VK_QUEUE_GRAPHICS_BIT
	QUEUE_COMPUTE_BIT  QueueFlags = vulkan.VK_QUEUE_COMPUTE_BIT

	SAMPLER_MIPMAP_MODE_NEAREST SamplerMipmapMode = vulkan.VK_SAMPLER_MIPMAP_MODE_NEAREST
	SAMPLER_MIPMAP_MODE_LINEAR  SamplerMipmapMode = vulkan.VK_SAMPLER_MIPMAP_MODE_LINEAR

	REMAINING_MIP_LEVELS = -1
)

// Vulkan macros that the generated bindings do not define.
const (
	remainingMipLevels   = ^uint32(0)      // ~0U
	remainingArrayLayers = ^uint32(0)      // ~0U
	wholeSize            = ^uint64(0)      // ~0ULL
	subpassExternal      = ^uint32(0)      // ~0U
	lodClampNone         = math.MaxFloat32 // FLT_MAX
	// VK_STRUCTURE_TYPE_WIN32_SURFACE_CREATE_INFO_KHR.
	structureTypeWin32SurfaceCreateInfo = 1000009000
)

var (
	vkOnce    sync.Once
	vkLoadErr error

	// vkGetInstanceProcAddr is the fallback resolver for entry points the
	// loader does not export directly.
	fvkGetInstanceProcAddr func(instance uintptr, pname *byte) uintptr
	// fvkCreateWin32SurfaceKHR is not part of the generated bindings.
	fvkCreateWin32SurfaceKHR func(instance uintptr, pCreateInfo, pAllocator, pSurface unsafe.Pointer) vulkan.VkResult
)

// vkProgs lists every entry point used by this file, in dependency order:
// vkGetInstanceProcAddr first so it can resolve the rest.
var vkProgs = []struct {
	name string
	fptr any
}{
	{"vkGetInstanceProcAddr", &fvkGetInstanceProcAddr},
	{"vkCreateWin32SurfaceKHR", &fvkCreateWin32SurfaceKHR},
	{"vkCreateInstance", &vulkan.VkCreateInstance},
	{"vkDestroyInstance", &vulkan.VkDestroyInstance},
	{"vkEnumeratePhysicalDevices", &vulkan.VkEnumeratePhysicalDevices},
	{"vkGetPhysicalDeviceQueueFamilyProperties", &vulkan.VkGetPhysicalDeviceQueueFamilyProperties},
	{"vkGetPhysicalDeviceFormatProperties", &vulkan.VkGetPhysicalDeviceFormatProperties},
	{"vkGetPhysicalDeviceProperties", &vulkan.VkGetPhysicalDeviceProperties},
	{"vkGetPhysicalDeviceMemoryProperties", &vulkan.VkGetPhysicalDeviceMemoryProperties},
	{"vkCreateDevice", &vulkan.VkCreateDevice},
	{"vkDestroyDevice", &vulkan.VkDestroyDevice},
	{"vkGetDeviceQueue", &vulkan.VkGetDeviceQueue},
	{"vkGetPhysicalDeviceSurfaceSupportKHR", &vulkan.VkGetPhysicalDeviceSurfaceSupportKHR},
	{"vkDestroySurfaceKHR", &vulkan.VkDestroySurfaceKHR},
	{"vkGetPhysicalDeviceSurfaceFormatsKHR", &vulkan.VkGetPhysicalDeviceSurfaceFormatsKHR},
	{"vkGetPhysicalDeviceSurfacePresentModesKHR", &vulkan.VkGetPhysicalDeviceSurfacePresentModesKHR},
	{"vkGetPhysicalDeviceSurfaceCapabilitiesKHR", &vulkan.VkGetPhysicalDeviceSurfaceCapabilitiesKHR},
	{"vkCreateSwapchainKHR", &vulkan.VkCreateSwapchainKHR},
	{"vkDestroySwapchainKHR", &vulkan.VkDestroySwapchainKHR},
	{"vkGetSwapchainImagesKHR", &vulkan.VkGetSwapchainImagesKHR},
	{"vkAcquireNextImageKHR", &vulkan.VkAcquireNextImageKHR},
	{"vkQueuePresentKHR", &vulkan.VkQueuePresentKHR},
	{"vkCreateImageView", &vulkan.VkCreateImageView},
	{"vkDestroyImageView", &vulkan.VkDestroyImageView},
	{"vkCreateFramebuffer", &vulkan.VkCreateFramebuffer},
	{"vkDestroyFramebuffer", &vulkan.VkDestroyFramebuffer},
	{"vkDeviceWaitIdle", &vulkan.VkDeviceWaitIdle},
	{"vkQueueWaitIdle", &vulkan.VkQueueWaitIdle},
	{"vkCreateSemaphore", &vulkan.VkCreateSemaphore},
	{"vkDestroySemaphore", &vulkan.VkDestroySemaphore},
	{"vkCreateRenderPass", &vulkan.VkCreateRenderPass},
	{"vkDestroyRenderPass", &vulkan.VkDestroyRenderPass},
	{"vkCreateCommandPool", &vulkan.VkCreateCommandPool},
	{"vkDestroyCommandPool", &vulkan.VkDestroyCommandPool},
	{"vkAllocateCommandBuffers", &vulkan.VkAllocateCommandBuffers},
	{"vkFreeCommandBuffers", &vulkan.VkFreeCommandBuffers},
	{"vkBeginCommandBuffer", &vulkan.VkBeginCommandBuffer},
	{"vkEndCommandBuffer", &vulkan.VkEndCommandBuffer},
	{"vkQueueSubmit", &vulkan.VkQueueSubmit},
	{"vkCmdBeginRenderPass", &vulkan.VkCmdBeginRenderPass},
	{"vkCmdEndRenderPass", &vulkan.VkCmdEndRenderPass},
	{"vkCmdCopyBuffer", &vulkan.VkCmdCopyBuffer},
	{"vkCmdCopyBufferToImage", &vulkan.VkCmdCopyBufferToImage},
	{"vkCmdPipelineBarrier", &vulkan.VkCmdPipelineBarrier},
	{"vkCmdPushConstants", &vulkan.VkCmdPushConstants},
	{"vkCmdBindPipeline", &vulkan.VkCmdBindPipeline},
	{"vkCmdBindVertexBuffers", &vulkan.VkCmdBindVertexBuffers},
	{"vkCmdSetViewport", &vulkan.VkCmdSetViewport},
	{"vkCmdBindIndexBuffer", &vulkan.VkCmdBindIndexBuffer},
	{"vkCmdDraw", &vulkan.VkCmdDraw},
	{"vkCmdDrawIndexed", &vulkan.VkCmdDrawIndexed},
	{"vkCmdBindDescriptorSets", &vulkan.VkCmdBindDescriptorSets},
	{"vkCmdCopyImageToBuffer", &vulkan.VkCmdCopyImageToBuffer},
	{"vkCmdDispatch", &vulkan.VkCmdDispatch},
	{"vkCreateImage", &vulkan.VkCreateImage},
	{"vkDestroyImage", &vulkan.VkDestroyImage},
	{"vkGetImageMemoryRequirements", &vulkan.VkGetImageMemoryRequirements},
	{"vkAllocateMemory", &vulkan.VkAllocateMemory},
	{"vkBindImageMemory", &vulkan.VkBindImageMemory},
	{"vkFreeMemory", &vulkan.VkFreeMemory},
	{"vkCreateSampler", &vulkan.VkCreateSampler},
	{"vkDestroySampler", &vulkan.VkDestroySampler},
	{"vkCreateBuffer", &vulkan.VkCreateBuffer},
	{"vkDestroyBuffer", &vulkan.VkDestroyBuffer},
	{"vkGetBufferMemoryRequirements", &vulkan.VkGetBufferMemoryRequirements},
	{"vkBindBufferMemory", &vulkan.VkBindBufferMemory},
	{"vkCreateShaderModule", &vulkan.VkCreateShaderModule},
	{"vkDestroyShaderModule", &vulkan.VkDestroyShaderModule},
	{"vkCreateGraphicsPipelines", &vulkan.VkCreateGraphicsPipelines},
	{"vkDestroyPipeline", &vulkan.VkDestroyPipeline},
	{"vkCreatePipelineLayout", &vulkan.VkCreatePipelineLayout},
	{"vkDestroyPipelineLayout", &vulkan.VkDestroyPipelineLayout},
	{"vkCreateDescriptorSetLayout", &vulkan.VkCreateDescriptorSetLayout},
	{"vkDestroyDescriptorSetLayout", &vulkan.VkDestroyDescriptorSetLayout},
	{"vkMapMemory", &vulkan.VkMapMemory},
	{"vkUnmapMemory", &vulkan.VkUnmapMemory},
	{"vkResetCommandBuffer", &vulkan.VkResetCommandBuffer},
	{"vkCreateDescriptorPool", &vulkan.VkCreateDescriptorPool},
	{"vkDestroyDescriptorPool", &vulkan.VkDestroyDescriptorPool},
	{"vkAllocateDescriptorSets", &vulkan.VkAllocateDescriptorSets},
	{"vkUpdateDescriptorSets", &vulkan.VkUpdateDescriptorSets},
	{"vkResetDescriptorPool", &vulkan.VkResetDescriptorPool},
	{"vkCmdBlitImage", &vulkan.VkCmdBlitImage},
	{"vkCmdCopyImage", &vulkan.VkCmdCopyImage},
	{"vkCreateComputePipelines", &vulkan.VkCreateComputePipelines},
	{"vkCreateFence", &vulkan.VkCreateFence},
	{"vkDestroyFence", &vulkan.VkDestroyFence},
	{"vkWaitForFences", &vulkan.VkWaitForFences},
	{"vkResetFences", &vulkan.VkResetFences},
}

func vkInit() error {
	vkOnce.Do(func() {
		dll := syscall.NewLazyDLL("vulkan-1.dll")
		if err := dll.Load(); err != nil {
			// Fall back to the ordinary DLL search order, which finds a
			// loader shipped next to the executable.
			dll = syscall.NewLazyDLL("vulkan-1.dll")
			if err := dll.Load(); err != nil {
				vkLoadErr = fmt.Errorf("vulkan: load vulkan-1.dll: %w", err)
				return
			}
		}
		for _, p := range vkProgs {
			proc := dll.NewProc(p.name)
			err := proc.Find()
			if err == nil {
				purego.RegisterFunc(p.fptr, proc.Addr())
				continue
			}
			// The loader does not export the symbol; ask vkGetInstanceProcAddr.
			if fvkGetInstanceProcAddr == nil || p.name == "vkGetInstanceProcAddr" {
				vkLoadErr = fmt.Errorf("vulkan: %s: %w", p.name, err)
				return
			}
			name := append([]byte(p.name), 0)
			addr := fvkGetInstanceProcAddr(0, &name[0])
			if addr == 0 {
				vkLoadErr = fmt.Errorf("vulkan: %s not found", p.name)
				return
			}
			purego.RegisterFunc(p.fptr, addr)
		}
	})
	return vkLoadErr
}

// cstringPtrs returns a pointer to a NUL terminated copy of each string. The
// pointers reference the copies, so keeping the result alive keeps the copies
// alive.
func cstringPtrs(strs []string) []unsafe.Pointer {
	ptrs := make([]unsafe.Pointer, len(strs))
	for i, s := range strs {
		b := append([]byte(s), 0)
		ptrs[i] = unsafe.Pointer(&b[0])
	}
	return ptrs
}

func CreateInstance(exts ...string) (Instance, error) {
	if err := vkInit(); err != nil {
		return nil, err
	}
	// VK_MAKE_API_VERSION macro.
	makeVer := func(variant, major, minor, patch uint32) uint32 {
		return (variant << 29) | (major << 22) | (minor << 12) | patch
	}
	appInf := vulkan.VkApplicationInfo{
		SType:      vulkan.VK_STRUCTURE_TYPE_APPLICATION_INFO,
		ApiVersion: makeVer(0, 1, 0, 0),
	}
	inf := vulkan.VkInstanceCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
		// pApplicationInfo may be omitted according to the spec, but the
		// Android emulator crashes without it.
		PApplicationInfo: unsafe.Pointer(&appInf),
	}
	var extPtrs []unsafe.Pointer
	if len(exts) > 0 {
		extPtrs = cstringPtrs(exts)
		inf.EnabledExtensionCount = uint32(len(exts))
		inf.PpEnabledExtensionNames = unsafe.Pointer(&extPtrs[0])
	}
	var inst Instance
	if err := vkErr(vulkan.VkCreateInstance(unsafe.Pointer(&inf), nil, unsafe.Pointer(&inst))); err != nil {
		return nil, fmt.Errorf("vulkan: vkCreateInstance: %w", err)
	}
	return inst, nil
}

func DestroyInstance(inst Instance) {
	vulkan.VkDestroyInstance(uintptr(inst), nil)
}

// CreateWin32Surface creates a VK_KHR_win32_surface surface for hwnd.
func CreateWin32Surface(inst Instance, hinstance, hwnd uintptr) (Surface, error) {
	if err := vkInit(); err != nil {
		return 0, err
	}
	inf := struct {
		sType     vulkan.VkStructureType
		pNext     unsafe.Pointer
		flags     uint32
		hinstance uintptr
		hwnd      uintptr
	}{
		sType:     structureTypeWin32SurfaceCreateInfo,
		hinstance: hinstance,
		hwnd:      hwnd,
	}
	var surf Surface
	if err := vkErr(fvkCreateWin32SurfaceKHR(uintptr(inst), unsafe.Pointer(&inf), nil, unsafe.Pointer(&surf))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateWin32SurfaceKHR: %w", err)
	}
	return surf, nil
}

// DeviceName returns the marketing name of a physical device, for logs.
func DeviceName(pd PhysicalDevice) string {
	if err := vkInit(); err != nil {
		return "unknown"
	}
	var props vulkan.VkPhysicalDeviceProperties
	vulkan.VkGetPhysicalDeviceProperties(uintptr(pd), unsafe.Pointer(&props))
	name := props.DeviceName[:]
	if i := bytes.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	return string(name)
}

func GetPhysicalDeviceQueueFamilyProperties(pd PhysicalDevice) []QueueFamilyProperties {
	var count uint32
	vulkan.VkGetPhysicalDeviceQueueFamilyProperties(uintptr(pd), unsafe.Pointer(&count), nil)
	if count == 0 {
		return nil
	}
	queues := make([]QueueFamilyProperties, count)
	vulkan.VkGetPhysicalDeviceQueueFamilyProperties(uintptr(pd), unsafe.Pointer(&count), unsafe.Pointer(&queues[0]))
	return queues
}

func EnumeratePhysicalDevices(inst Instance) ([]PhysicalDevice, error) {
	var count uint32
	if err := vkErr(vulkan.VkEnumeratePhysicalDevices(uintptr(inst), unsafe.Pointer(&count), nil)); err != nil {
		return nil, fmt.Errorf("vulkan: vkEnumeratePhysicalDevices: %w", err)
	}
	if count == 0 {
		return nil, nil
	}
	devs := make([]PhysicalDevice, count)
	if err := vkErr(vulkan.VkEnumeratePhysicalDevices(uintptr(inst), unsafe.Pointer(&count), unsafe.Pointer(&devs[0]))); err != nil {
		return nil, fmt.Errorf("vulkan: vkEnumeratePhysicalDevices: %w", err)
	}
	return devs, nil
}

func ChoosePhysicalDevice(inst Instance, surf Surface) (PhysicalDevice, int, error) {
	devs, err := EnumeratePhysicalDevices(inst)
	if err != nil {
		return nil, 0, err
	}
	for _, pd := range devs {
		var props vulkan.VkPhysicalDeviceProperties
		vulkan.VkGetPhysicalDeviceProperties(uintptr(pd), unsafe.Pointer(&props))
		// The lavapipe software implementation doesn't work well rendering to a surface.
		// See https://gitlab.freedesktop.org/mesa/mesa/-/issues/5473.
		if surf != 0 && props.DeviceType == vulkan.VK_PHYSICAL_DEVICE_TYPE_CPU {
			continue
		}
		const caps = vulkan.VK_QUEUE_GRAPHICS_BIT | vulkan.VK_QUEUE_COMPUTE_BIT
		queueIdx, ok, err := chooseQueue(pd, surf, caps)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			continue
		}
		if surf != 0 {
			_, fmtFound, err := chooseFormat(pd, surf)
			if err != nil {
				return nil, 0, err
			}
			_, modFound, err := choosePresentMode(pd, surf)
			if err != nil {
				return nil, 0, err
			}
			if !fmtFound || !modFound {
				continue
			}
		}
		return pd, queueIdx, nil
	}
	return nil, 0, errors.New("vulkan: no suitable device found")
}

func CreateDeviceAndQueue(pd PhysicalDevice, queueIdx int, exts ...string) (Device, error) {
	priority := float32(1.0)
	qinf := vulkan.VkDeviceQueueCreateInfo{
		SType:            vulkan.VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,
		QueueCount:       1,
		QueueFamilyIndex: uint32(queueIdx),
		PQueuePriorities: unsafe.Pointer(&priority),
	}
	inf := vulkan.VkDeviceCreateInfo{
		SType:                 vulkan.VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,
		QueueCreateInfoCount:  1,
		PQueueCreateInfos:     unsafe.Pointer(&qinf),
		EnabledExtensionCount: uint32(len(exts)),
	}
	var extPtrs []unsafe.Pointer
	if len(exts) > 0 {
		extPtrs = cstringPtrs(exts)
		inf.PpEnabledExtensionNames = unsafe.Pointer(&extPtrs[0])
	}
	var dev Device
	if err := vkErr(vulkan.VkCreateDevice(uintptr(pd), unsafe.Pointer(&inf), nil, unsafe.Pointer(&dev))); err != nil {
		return nil, fmt.Errorf("vulkan: vkCreateDevice: %w", err)
	}
	return dev, nil
}

func GetDeviceQueue(d Device, queueFamily, queueIndex int) Queue {
	var queue Queue
	vulkan.VkGetDeviceQueue(uintptr(d), uint32(queueFamily), uint32(queueIndex), unsafe.Pointer(&queue))
	return queue
}

func GetPhysicalDeviceSurfaceCapabilities(pd PhysicalDevice, surf Surface) (SurfaceCapabilities, error) {
	var caps SurfaceCapabilities
	err := vkErr(vulkan.VkGetPhysicalDeviceSurfaceCapabilitiesKHR(uintptr(pd), surf, unsafe.Pointer(&caps)))
	if err != nil {
		return SurfaceCapabilities{}, fmt.Errorf("vulkan: vkGetPhysicalDeviceSurfaceCapabilitiesKHR: %w", err)
	}
	return caps, nil
}

func CreateSwapchain(pd PhysicalDevice, d Device, surf Surface, width, height int, old Swapchain) (Swapchain, []Image, Format, error) {
	caps, err := GetPhysicalDeviceSurfaceCapabilities(pd, surf)
	if err != nil {
		return 0, nil, 0, err
	}
	mode, modeOK, err := choosePresentMode(pd, surf)
	if err != nil {
		return 0, nil, 0, err
	}
	format, fmtOK, err := chooseFormat(pd, surf)
	if err != nil {
		return 0, nil, 0, err
	}
	if !modeOK || !fmtOK {
		// This shouldn't happen because CreateDeviceAndQueue found at least
		// one valid format and present mode.
		return 0, nil, 0, errors.New("vulkan: no valid format and present mode found")
	}
	// Find supported alpha composite mode. It doesn't matter which one, because rendering is
	// always opaque.
	alphaComp := vulkan.VkCompositeAlphaFlagBitsKHR(1)
	for caps.SupportedCompositeAlpha&vulkan.VkCompositeAlphaFlagsKHR(alphaComp) == 0 {
		alphaComp <<= 1
	}
	trans := vulkan.VkSurfaceTransformFlagBitsKHR(vulkan.VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR)
	if caps.SupportedTransforms&vulkan.VkSurfaceTransformFlagsKHR(trans) == 0 {
		return 0, nil, 0, errors.New("vulkan: VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR not supported")
	}
	inf := vulkan.VkSwapchainCreateInfoKHR{
		SType:            vulkan.VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR,
		Surface:          surf,
		MinImageCount:    caps.MinImageCount,
		ImageFormat:      format.Format,
		ImageColorSpace:  format.ColorSpace,
		ImageExtent:      vulkan.VkExtent2D{Width: uint32(width), Height: uint32(height)},
		ImageArrayLayers: 1,
		ImageUsage:       vulkan.VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT,
		ImageSharingMode: vulkan.VK_SHARING_MODE_EXCLUSIVE,
		PreTransform:     trans,
		PresentMode:      mode,
		CompositeAlpha:   alphaComp,
		Clipped:          1,
		OldSwapchain:     old,
	}
	var swchain Swapchain
	if err := vkErr(vulkan.VkCreateSwapchainKHR(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&swchain))); err != nil {
		return 0, nil, 0, fmt.Errorf("vulkan: vkCreateSwapchainKHR: %w", err)
	}
	var count uint32
	if err := vkErr(vulkan.VkGetSwapchainImagesKHR(uintptr(d), swchain, unsafe.Pointer(&count), nil)); err != nil {
		DestroySwapchain(d, swchain)
		return 0, nil, 0, fmt.Errorf("vulkan: vkGetSwapchainImagesKHR: %w", err)
	}
	if count == 0 {
		DestroySwapchain(d, swchain)
		return 0, nil, 0, errors.New("vulkan: vkGetSwapchainImagesKHR returned no images")
	}
	imgs := make([]Image, count)
	if err := vkErr(vulkan.VkGetSwapchainImagesKHR(uintptr(d), swchain, unsafe.Pointer(&count), unsafe.Pointer(&imgs[0]))); err != nil {
		DestroySwapchain(d, swchain)
		return 0, nil, 0, fmt.Errorf("vulkan: vkGetSwapchainImagesKHR: %w", err)
	}
	return swchain, imgs, format.Format, nil
}

func DestroySwapchain(d Device, swchain Swapchain) {
	vulkan.VkDestroySwapchainKHR(uintptr(d), swchain, nil)
}

func AcquireNextImage(d Device, swchain Swapchain, sem Semaphore, fence Fence) (int, error) {
	var idx uint32
	res := vulkan.VkAcquireNextImageKHR(uintptr(d), swchain, math.MaxUint64, sem, fence, unsafe.Pointer(&idx))
	if err := vkErr(res); err != nil {
		return 0, fmt.Errorf("vulkan: vkAcquireNextImageKHR: %w", err)
	}
	return int(idx), nil
}

func PresentQueue(q Queue, swchain Swapchain, sem Semaphore, imgIdx int) error {
	cidx := uint32(imgIdx)
	inf := vulkan.VkPresentInfoKHR{
		SType:          vulkan.VK_STRUCTURE_TYPE_PRESENT_INFO_KHR,
		SwapchainCount: 1,
		PSwapchains:    unsafe.Pointer(&swchain),
		PImageIndices:  unsafe.Pointer(&cidx),
	}
	if sem != 0 {
		inf.WaitSemaphoreCount = 1
		inf.PWaitSemaphores = unsafe.Pointer(&sem)
	}
	if err := vkErr(vulkan.VkQueuePresentKHR(uintptr(q), unsafe.Pointer(&inf))); err != nil {
		return fmt.Errorf("vulkan: vkQueuePresentKHR: %w", err)
	}
	return nil
}

func CreateImageView(d Device, img Image, format Format) (ImageView, error) {
	return CreateImageViewEx(d, img, format, IMAGE_ASPECT_COLOR_BIT)
}

// CreateImageViewEx is CreateImageView for an arbitrary image aspect, which is
// what a depth buffer view needs: the color aspect is not present on it.
func CreateImageViewEx(d Device, img Image, format Format, aspect uint32) (ImageView, error) {
	inf := vulkan.VkImageViewCreateInfo{
		SType:    vulkan.VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
		Image:    img,
		ViewType: vulkan.VK_IMAGE_VIEW_TYPE_2D,
		Format:   format,
		SubresourceRange: vulkan.VkImageSubresourceRange{
			AspectMask: aspect,
			LevelCount: remainingMipLevels,
			LayerCount: remainingArrayLayers,
		},
	}
	var view ImageView
	if err := vkErr(vulkan.VkCreateImageView(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&view))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateImageView: %w", err)
	}
	return view, nil
}

func DestroyImageView(d Device, view ImageView) {
	vulkan.VkDestroyImageView(uintptr(d), view, nil)
}

func CreateRenderPass(d Device, format Format, loadOp AttachmentLoadOp, initialLayout, finalLayout ImageLayout, passDeps []SubpassDependency) (RenderPass, error) {
	return CreateRenderPassEx(d, RenderPassDesc{
		ColorFormat:   format,
		LoadOp:        loadOp,
		InitialLayout: initialLayout,
		FinalLayout:   finalLayout,
		Dependencies:  passDeps,
	})
}

// RenderPassDesc describes a single subpass render pass with one color
// attachment and an optional depth attachment.
type RenderPassDesc struct {
	// ColorFormat is the format of the color attachment.
	ColorFormat Format
	// DepthFormat adds a depth attachment in that format. Zero leaves the
	// pass without one.
	DepthFormat Format
	LoadOp      AttachmentLoadOp
	// InitialLayout and FinalLayout bracket the color attachment; the
	// subpass itself always works on it in COLOR_ATTACHMENT_OPTIMAL. The
	// depth attachment stays in DEPTH_STENCIL_ATTACHMENT_OPTIMAL on both
	// sides, so a pass that clears it never needs a transition of its own.
	InitialLayout, FinalLayout ImageLayout
	Dependencies               []SubpassDependency
}

func CreateRenderPassEx(d Device, desc RenderPassDesc) (RenderPass, error) {
	color := vulkan.VkAttachmentDescription{
		Format:         desc.ColorFormat,
		Samples:        vulkan.VK_SAMPLE_COUNT_1_BIT,
		LoadOp:         desc.LoadOp,
		StoreOp:        vulkan.VK_ATTACHMENT_STORE_OP_STORE,
		StencilLoadOp:  vulkan.VK_ATTACHMENT_LOAD_OP_DONT_CARE,
		StencilStoreOp: vulkan.VK_ATTACHMENT_STORE_OP_DONT_CARE,
		InitialLayout:  desc.InitialLayout,
		FinalLayout:    desc.FinalLayout,
	}
	colorRef := vulkan.VkAttachmentReference{
		Attachment: 0,
		Layout:     vulkan.VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
	}
	sub := vulkan.VkSubpassDescription{
		PipelineBindPoint:    vulkan.VK_PIPELINE_BIND_POINT_GRAPHICS,
		ColorAttachmentCount: 1,
		PColorAttachments:    unsafe.Pointer(&colorRef),
	}
	attachments := []vulkan.VkAttachmentDescription{color}
	var depthRef vulkan.VkAttachmentReference
	if desc.DepthFormat != 0 {
		depthRef = vulkan.VkAttachmentReference{
			Attachment: 1,
			Layout:     vulkan.VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		}
		attachments = append(attachments, vulkan.VkAttachmentDescription{
			Format:         desc.DepthFormat,
			Samples:        vulkan.VK_SAMPLE_COUNT_1_BIT,
			LoadOp:         desc.LoadOp,
			StoreOp:        vulkan.VK_ATTACHMENT_STORE_OP_STORE,
			StencilLoadOp:  vulkan.VK_ATTACHMENT_LOAD_OP_DONT_CARE,
			StencilStoreOp: vulkan.VK_ATTACHMENT_STORE_OP_DONT_CARE,
			InitialLayout:  vulkan.VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
			FinalLayout:    vulkan.VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		})
		sub.PDepthStencilAttachment = unsafe.Pointer(&depthRef)
	}

	inf := vulkan.VkRenderPassCreateInfo{
		SType:           vulkan.VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO,
		AttachmentCount: uint32(len(attachments)),
		PAttachments:    unsafe.Pointer(&attachments[0]),
		SubpassCount:    1,
		PSubpasses:      unsafe.Pointer(&sub),
	}
	if n := len(desc.Dependencies); n > 0 {
		inf.DependencyCount = uint32(n)
		inf.PDependencies = unsafe.Pointer(&desc.Dependencies[0])
	}

	var pass RenderPass
	if err := vkErr(vulkan.VkCreateRenderPass(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&pass))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateRenderPass: %w", err)
	}
	return pass, nil
}

func DestroyRenderPass(d Device, r RenderPass) {
	vulkan.VkDestroyRenderPass(uintptr(d), r, nil)
}

func CreateFramebuffer(d Device, rp RenderPass, view ImageView, width, height int) (Framebuffer, error) {
	return CreateFramebufferEx(d, rp, []ImageView{view}, width, height)
}

// CreateFramebufferEx builds a framebuffer from a list of views in the order
// the render pass declares its attachments: color first, then depth.
func CreateFramebufferEx(d Device, rp RenderPass, views []ImageView, width, height int) (Framebuffer, error) {
	if len(views) == 0 {
		return 0, errors.New("vulkan: framebuffer needs at least one attachment")
	}
	inf := vulkan.VkFramebufferCreateInfo{
		SType:           vulkan.VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO,
		RenderPass:      rp,
		AttachmentCount: uint32(len(views)),
		PAttachments:    unsafe.Pointer(&views[0]),
		Width:           uint32(width),
		Height:          uint32(height),
		Layers:          1,
	}
	var fbo Framebuffer
	if err := vkErr(vulkan.VkCreateFramebuffer(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&fbo))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateFramebuffer: %w", err)
	}
	return fbo, nil
}

func DestroyFramebuffer(d Device, f Framebuffer) {
	vulkan.VkDestroyFramebuffer(uintptr(d), f, nil)
}

func DeviceWaitIdle(d Device) error {
	if err := vkErr(vulkan.VkDeviceWaitIdle(uintptr(d))); err != nil {
		return fmt.Errorf("vulkan: vkDeviceWaitIdle: %w", err)
	}
	return nil
}

func QueueWaitIdle(q Queue) error {
	if err := vkErr(vulkan.VkQueueWaitIdle(uintptr(q))); err != nil {
		return fmt.Errorf("vulkan: vkQueueWaitIdle: %w", err)
	}
	return nil
}

func CreateSemaphore(d Device) (Semaphore, error) {
	inf := vulkan.VkSemaphoreCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO,
	}
	var sem Semaphore
	err := vkErr(vulkan.VkCreateSemaphore(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&sem)))
	if err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateSemaphore: %w", err)
	}
	return sem, err
}

func DestroySemaphore(d Device, sem Semaphore) {
	vulkan.VkDestroySemaphore(uintptr(d), sem, nil)
}

func DestroyDevice(dev Device) {
	vulkan.VkDestroyDevice(uintptr(dev), nil)
}

func DestroySurface(inst Instance, s Surface) {
	vulkan.VkDestroySurfaceKHR(uintptr(inst), s, nil)
}

func CreateCommandPool(d Device, queueIndex int) (CommandPool, error) {
	inf := vulkan.VkCommandPoolCreateInfo{
		SType:            vulkan.VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,
		QueueFamilyIndex: uint32(queueIndex),
		Flags:            vulkan.VK_COMMAND_POOL_CREATE_TRANSIENT_BIT | vulkan.VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT,
	}

	var pool CommandPool
	if err := vkErr(vulkan.VkCreateCommandPool(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&pool))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateCommandPool: %w", err)
	}
	return pool, nil
}

func DestroyCommandPool(d Device, pool CommandPool) {
	vulkan.VkDestroyCommandPool(uintptr(d), pool, nil)
}

func AllocateCommandBuffer(d Device, pool CommandPool) (CommandBuffer, error) {
	inf := vulkan.VkCommandBufferAllocateInfo{
		SType:              vulkan.VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
		CommandPool:        pool,
		Level:              vulkan.VK_COMMAND_BUFFER_LEVEL_PRIMARY,
		CommandBufferCount: 1,
	}

	var buf CommandBuffer
	if err := vkErr(vulkan.VkAllocateCommandBuffers(uintptr(d), unsafe.Pointer(&inf), unsafe.Pointer(&buf))); err != nil {
		return nil, fmt.Errorf("vulkan: vkAllocateCommandBuffers: %w", err)
	}
	return buf, nil
}

func FreeCommandBuffers(d Device, pool CommandPool, bufs ...CommandBuffer) {
	if len(bufs) == 0 {
		return
	}
	vulkan.VkFreeCommandBuffers(uintptr(d), pool, uint32(len(bufs)), unsafe.Pointer(&bufs[0]))
}

func BeginCommandBuffer(buf CommandBuffer) error {
	inf := vulkan.VkCommandBufferBeginInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO,
		Flags: vulkan.VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT,
	}
	if err := vkErr(vulkan.VkBeginCommandBuffer(uintptr(buf), unsafe.Pointer(&inf))); err != nil {
		return fmt.Errorf("vulkan: vkBeginCommandBuffer: %w", err)
	}
	return nil
}

func EndCommandBuffer(buf CommandBuffer) error {
	if err := vkErr(vulkan.VkEndCommandBuffer(uintptr(buf))); err != nil {
		return fmt.Errorf("vulkan: vkEndCommandBuffer: %w", err)
	}
	return nil
}

func QueueSubmit(q Queue, buf CommandBuffer, waitSems []Semaphore, waitStages []PipelineStageFlags, sigSems []Semaphore, fence Fence) error {
	inf := vulkan.VkSubmitInfo{
		SType:              vulkan.VK_STRUCTURE_TYPE_SUBMIT_INFO,
		CommandBufferCount: 1,
		PCommandBuffers:    unsafe.Pointer(&buf),
	}
	if len(waitSems) > 0 {
		if len(waitSems) != len(waitStages) {
			panic("len(waitSems) != len(waitStages)")
		}
		inf.WaitSemaphoreCount = uint32(len(waitSems))
		inf.PWaitSemaphores = unsafe.Pointer(&waitSems[0])
		inf.PWaitDstStageMask = unsafe.Pointer(&waitStages[0])
	}
	if len(sigSems) > 0 {
		inf.SignalSemaphoreCount = uint32(len(sigSems))
		inf.PSignalSemaphores = unsafe.Pointer(&sigSems[0])
	}
	if err := vkErr(vulkan.VkQueueSubmit(uintptr(q), 1, unsafe.Pointer(&inf), fence)); err != nil {
		return fmt.Errorf("vulkan: vkQueueSubmit: %w", err)
	}
	return nil
}

func CmdBeginRenderPass(buf CommandBuffer, rp RenderPass, fbo Framebuffer, width, height int, clearCol [4]float32) {
	CmdBeginRenderPassEx(buf, rp, fbo, width, height, clearCol, false, 0)
}

// CmdBeginRenderPassEx begins a pass, clearing the color attachment to
// clearCol and, when hasDepth is set, the depth attachment to clearDepth. The
// clear values are in the order the render pass declares its attachments.
func CmdBeginRenderPassEx(buf CommandBuffer, rp RenderPass, fbo Framebuffer, width, height int, clearCol [4]float32, hasDepth bool, clearDepth float32) {
	clears := make([]vulkan.VkClearValue, 1, 2)
	*clears[0].AsColor().AsFloat32() = clearCol
	if hasDepth {
		var depthClear vulkan.VkClearValue
		depthClear.AsDepthStencil().Depth = clearDepth
		depthClear.AsDepthStencil().Stencil = 0
		clears = append(clears, depthClear)
	}
	inf := vulkan.VkRenderPassBeginInfo{
		SType:           vulkan.VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO,
		RenderPass:      rp,
		Framebuffer:     fbo,
		RenderArea:      vulkan.VkRect2D{Extent: vulkan.VkExtent2D{Width: uint32(width), Height: uint32(height)}},
		ClearValueCount: uint32(len(clears)),
		PClearValues:    unsafe.Pointer(&clears[0]),
	}
	vulkan.VkCmdBeginRenderPass(uintptr(buf), unsafe.Pointer(&inf), vulkan.VK_SUBPASS_CONTENTS_INLINE)
}

func CmdEndRenderPass(buf CommandBuffer) {
	vulkan.VkCmdEndRenderPass(uintptr(buf))
}

func CmdCopyBuffer(cmdBuf CommandBuffer, src, dst Buffer, srcOff, dstOff, size int) {
	region := vulkan.VkBufferCopy{
		SrcOffset: DeviceSize(srcOff),
		DstOffset: DeviceSize(dstOff),
		Size:      DeviceSize(size),
	}
	vulkan.VkCmdCopyBuffer(uintptr(cmdBuf), src, dst, 1, unsafe.Pointer(&region))
}

func CmdCopyBufferToImage(cmdBuf CommandBuffer, src Buffer, dst Image, layout ImageLayout, copy BufferImageCopy) {
	vulkan.VkCmdCopyBufferToImage(uintptr(cmdBuf), src, dst, layout, 1, unsafe.Pointer(&copy))
}

func CmdPipelineBarrier(cmdBuf CommandBuffer, srcStage, dstStage PipelineStageFlags, flags DependencyFlags, memBarriers []MemoryBarrier, bufBarriers []BufferMemoryBarrier, imgBarriers []ImageMemoryBarrier) {
	var memPtr unsafe.Pointer
	if len(memBarriers) > 0 {
		memPtr = unsafe.Pointer(&memBarriers[0])
	}
	var bufPtr unsafe.Pointer
	if len(bufBarriers) > 0 {
		bufPtr = unsafe.Pointer(&bufBarriers[0])
	}
	var imgPtr unsafe.Pointer
	if len(imgBarriers) > 0 {
		imgPtr = unsafe.Pointer(&imgBarriers[0])
	}
	vulkan.VkCmdPipelineBarrier(uintptr(cmdBuf), srcStage, dstStage, flags,
		uint32(len(memBarriers)), memPtr,
		uint32(len(bufBarriers)), bufPtr,
		uint32(len(imgBarriers)), imgPtr)
}

func CmdPushConstants(cmdBuf CommandBuffer, layout PipelineLayout, stages ShaderStageFlags, offset int, data []byte) {
	vulkan.VkCmdPushConstants(uintptr(cmdBuf), layout, stages, uint32(offset), uint32(len(data)),
		unsafe.Pointer(unsafe.SliceData(data)))
}

func CmdBindPipeline(cmdBuf CommandBuffer, bindPoint PipelineBindPoint, pipe Pipeline) {
	vulkan.VkCmdBindPipeline(uintptr(cmdBuf), bindPoint, pipe)
}

func CmdBindVertexBuffers(cmdBuf CommandBuffer, first int, buffers []Buffer, sizes []DeviceSize) {
	if len(buffers) == 0 {
		return
	}
	vulkan.VkCmdBindVertexBuffers(uintptr(cmdBuf), uint32(first), uint32(len(buffers)), unsafe.Pointer(&buffers[0]), unsafe.Pointer(&sizes[0]))
}

func CmdSetViewport(cmdBuf CommandBuffer, first int, viewports ...Viewport) {
	if len(viewports) == 0 {
		return
	}
	vulkan.VkCmdSetViewport(uintptr(cmdBuf), uint32(first), uint32(len(viewports)), unsafe.Pointer(&viewports[0]))
}

func CmdBindIndexBuffer(cmdBuf CommandBuffer, buffer Buffer, offset int, typ IndexType) {
	vulkan.VkCmdBindIndexBuffer(uintptr(cmdBuf), buffer, DeviceSize(offset), typ)
}

func CmdDraw(cmdBuf CommandBuffer, vertCount, instCount, firstVert, firstInst int) {
	vulkan.VkCmdDraw(uintptr(cmdBuf), uint32(vertCount), uint32(instCount), uint32(firstVert), uint32(firstInst))
}

func CmdDrawIndexed(cmdBuf CommandBuffer, idxCount, instCount, firstIdx, vertOff, firstInst int) {
	vulkan.VkCmdDrawIndexed(uintptr(cmdBuf), uint32(idxCount), uint32(instCount), uint32(firstIdx), int32(vertOff), uint32(firstInst))
}

func GetPhysicalDeviceFormatProperties(physDev PhysicalDevice, format Format) FormatFeatureFlags {
	var props vulkan.VkFormatProperties
	vulkan.VkGetPhysicalDeviceFormatProperties(uintptr(physDev), format, unsafe.Pointer(&props))
	return FormatFeatureFlags(props.OptimalTilingFeatures)
}

func CmdBindDescriptorSets(cmdBuf CommandBuffer, point PipelineBindPoint, layout PipelineLayout, firstSet int, sets []DescriptorSet) {
	if len(sets) == 0 {
		return
	}
	vulkan.VkCmdBindDescriptorSets(uintptr(cmdBuf), point, layout, uint32(firstSet), uint32(len(sets)), unsafe.Pointer(&sets[0]), 0, nil)
}

func CmdBlitImage(cmdBuf CommandBuffer, src Image, srcLayout ImageLayout, dst Image, dstLayout ImageLayout, regions []ImageBlit, filter Filter) {
	if len(regions) == 0 {
		return
	}
	vulkan.VkCmdBlitImage(uintptr(cmdBuf), src, srcLayout, dst, dstLayout, uint32(len(regions)), unsafe.Pointer(&regions[0]), filter)
}

func CmdCopyImage(cmdBuf CommandBuffer, src Image, srcLayout ImageLayout, dst Image, dstLayout ImageLayout, regions []ImageCopy) {
	if len(regions) == 0 {
		return
	}
	vulkan.VkCmdCopyImage(uintptr(cmdBuf), src, srcLayout, dst, dstLayout, uint32(len(regions)), unsafe.Pointer(&regions[0]))
}

func CmdCopyImageToBuffer(cmdBuf CommandBuffer, src Image, srcLayout ImageLayout, dst Buffer, regions []BufferImageCopy) {
	if len(regions) == 0 {
		return
	}
	vulkan.VkCmdCopyImageToBuffer(uintptr(cmdBuf), src, srcLayout, dst, uint32(len(regions)), unsafe.Pointer(&regions[0]))
}

func CmdDispatch(cmdBuf CommandBuffer, x, y, z int) {
	vulkan.VkCmdDispatch(uintptr(cmdBuf), uint32(x), uint32(y), uint32(z))
}

func CreateImage(pd PhysicalDevice, d Device, format Format, width, height, mipmaps int, usage ImageUsageFlags) (Image, DeviceMemory, error) {
	inf := vulkan.VkImageCreateInfo{
		SType:     vulkan.VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
		ImageType: vulkan.VK_IMAGE_TYPE_2D,
		Format:    format,
		Extent: vulkan.VkExtent3D{
			Width:  uint32(width),
			Height: uint32(height),
			Depth:  1,
		},
		MipLevels:     uint32(mipmaps),
		ArrayLayers:   1,
		Samples:       vulkan.VK_SAMPLE_COUNT_1_BIT,
		Tiling:        vulkan.VK_IMAGE_TILING_OPTIMAL,
		Usage:         usage,
		InitialLayout: vulkan.VK_IMAGE_LAYOUT_UNDEFINED,
	}
	var img Image
	if err := vkErr(vulkan.VkCreateImage(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&img))); err != nil {
		return 0, 0, fmt.Errorf("vulkan: vkCreateImage: %w", err)
	}
	var memReqs vulkan.VkMemoryRequirements
	vulkan.VkGetImageMemoryRequirements(uintptr(d), img, unsafe.Pointer(&memReqs))

	memIdx, found := findMemoryTypeIndex(pd, memReqs.MemoryTypeBits, vulkan.VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT)
	if !found {
		DestroyImage(d, img)
		return 0, 0, errors.New("vulkan: no memory type suitable for images found")
	}

	memInf := vulkan.VkMemoryAllocateInfo{
		SType:           vulkan.VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: uint32(memIdx),
	}
	var imgMem DeviceMemory
	if err := vkErr(vulkan.VkAllocateMemory(uintptr(d), unsafe.Pointer(&memInf), nil, unsafe.Pointer(&imgMem))); err != nil {
		DestroyImage(d, img)
		return 0, 0, fmt.Errorf("vulkan: vkAllocateMemory: %w", err)
	}

	if err := vkErr(vulkan.VkBindImageMemory(uintptr(d), img, imgMem, 0)); err != nil {
		FreeMemory(d, imgMem)
		DestroyImage(d, img)
		return 0, 0, fmt.Errorf("vulkan: vkBindImageMemory: %w", err)
	}
	return img, imgMem, nil
}

func DestroyImage(d Device, img Image) {
	vulkan.VkDestroyImage(uintptr(d), img, nil)
}

func FreeMemory(d Device, mem DeviceMemory) {
	vulkan.VkFreeMemory(uintptr(d), mem, nil)
}

func CreateSampler(d Device, minFilter, magFilter Filter, mipmapMode SamplerMipmapMode) (Sampler, error) {
	inf := vulkan.VkSamplerCreateInfo{
		SType:        vulkan.VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO,
		MinFilter:    minFilter,
		MagFilter:    magFilter,
		MipmapMode:   mipmapMode,
		MaxLod:       lodClampNone,
		AddressModeU: vulkan.VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE,
		AddressModeV: vulkan.VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE,
	}
	var s Sampler
	if err := vkErr(vulkan.VkCreateSampler(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&s))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateSampler: %w", err)
	}
	return s, nil
}

func DestroySampler(d Device, sampler Sampler) {
	vulkan.VkDestroySampler(uintptr(d), sampler, nil)
}

func CreateBuffer(pd PhysicalDevice, d Device, size int, usage BufferUsageFlags, props MemoryPropertyFlags) (Buffer, DeviceMemory, error) {
	inf := vulkan.VkBufferCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO,
		Size:  DeviceSize(size),
		Usage: usage,
	}
	var buf Buffer
	if err := vkErr(vulkan.VkCreateBuffer(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&buf))); err != nil {
		return 0, 0, fmt.Errorf("vulkan: vkCreateBuffer: %w", err)
	}

	var memReqs vulkan.VkMemoryRequirements
	vulkan.VkGetBufferMemoryRequirements(uintptr(d), buf, unsafe.Pointer(&memReqs))

	memIdx, found := findMemoryTypeIndex(pd, memReqs.MemoryTypeBits, props)
	if !found {
		DestroyBuffer(d, buf)
		return 0, 0, errors.New("vulkan: no memory suitable for buffers found")
	}
	memInf := vulkan.VkMemoryAllocateInfo{
		SType:           vulkan.VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: uint32(memIdx),
	}

	var mem DeviceMemory
	if err := vkErr(vulkan.VkAllocateMemory(uintptr(d), unsafe.Pointer(&memInf), nil, unsafe.Pointer(&mem))); err != nil {
		DestroyBuffer(d, buf)
		return 0, 0, fmt.Errorf("vulkan: vkAllocateMemory: %w", err)
	}

	if err := vkErr(vulkan.VkBindBufferMemory(uintptr(d), buf, mem, 0)); err != nil {
		FreeMemory(d, mem)
		DestroyBuffer(d, buf)
		return 0, 0, fmt.Errorf("vulkan: vkBindBufferMemory: %w", err)
	}
	return buf, mem, nil
}

func DestroyBuffer(d Device, buf Buffer) {
	vulkan.VkDestroyBuffer(uintptr(d), buf, nil)
}

func CreateShaderModule(d Device, spirv string) (ShaderModule, error) {
	inf := vulkan.VkShaderModuleCreateInfo{
		SType:    vulkan.VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO,
		CodeSize: uintptr(len(spirv)),
		PCode:    unsafe.Pointer(unsafe.StringData(spirv)),
	}

	var mod ShaderModule
	if err := vkErr(vulkan.VkCreateShaderModule(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&mod))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateShaderModule: %w", err)
	}
	return mod, nil
}

func DestroyShaderModule(d Device, mod ShaderModule) {
	vulkan.VkDestroyShaderModule(uintptr(d), mod, nil)
}

func CreateGraphicsPipeline(d Device, pass RenderPass, vmod, fmod ShaderModule, blend bool, srcFactor, dstFactor BlendFactor, topology PrimitiveTopology, bindings []VertexInputBindingDescription, attrs []VertexInputAttributeDescription, layout PipelineLayout) (Pipeline, error) {
	return CreateGraphicsPipelineEx(d, pass, PipelineDesc{
		Vertex:    vmod,
		Fragment:  fmod,
		Blend:     blend,
		SrcFactor: srcFactor,
		DstFactor: dstFactor,
		Topology:  topology,
		Bindings:  bindings,
		Attrs:     attrs,
		Layout:    layout,
	})
}

// DepthState is the depth configuration of a pipeline. A nil *DepthState in
// PipelineDesc means the render pass has no depth attachment, and the
// pipeline then carries no depth-stencil state at all.
type DepthState struct {
	// Test compares fragments against the bound depth buffer.
	Test bool
	// Write keeps the depth of the fragments that pass the test.
	Write bool
}

// PipelineDesc is the struct form of CreateGraphicsPipeline, extended with
// the rasterizer and depth state a custom pass needs.
type PipelineDesc struct {
	Vertex, Fragment     ShaderModule
	Blend                bool
	SrcFactor, DstFactor BlendFactor
	Topology             PrimitiveTopology
	Bindings             []VertexInputBindingDescription
	Attrs                []VertexInputAttributeDescription
	Layout               PipelineLayout
	// Cull selects which windings are discarded.
	Cull CullMode
	// Depth is the depth state, or nil when the pass has no depth buffer.
	Depth *DepthState
}

// CreateGraphicsPipelineEx compiles a pipeline with the rasterizer and depth
// state a custom pass needs. Front faces are declared clockwise, the
// convention Direct3D uses, so a caller that culls the front faces to match a
// Direct3D pipeline gets the same triangles on screen — provided it also
// brings the projection into Vulkan's downward Y, which flips positions back
// to what Direct3D had.
func CreateGraphicsPipelineEx(d Device, pass RenderPass, desc PipelineDesc) (Pipeline, error) {
	main := append([]byte("main"), 0)
	pName := unsafe.Pointer(&main[0])
	stages := [2]vulkan.VkPipelineShaderStageCreateInfo{
		{
			SType:  vulkan.VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO,
			Stage:  vulkan.VK_SHADER_STAGE_VERTEX_BIT,
			Module: desc.Vertex,
			PName:  pName,
		},
		{
			SType:  vulkan.VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO,
			Stage:  vulkan.VK_SHADER_STAGE_FRAGMENT_BIT,
			Module: desc.Fragment,
			PName:  pName,
		},
	}
	dynStates := []vulkan.VkDynamicState{vulkan.VK_DYNAMIC_STATE_VIEWPORT}
	dynInf := vulkan.VkPipelineDynamicStateCreateInfo{
		SType:             vulkan.VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO,
		DynamicStateCount: uint32(len(dynStates)),
		PDynamicStates:    unsafe.Pointer(&dynStates[0]),
	}
	const maxDim = 0x7fffffff
	scissors := []vulkan.VkRect2D{{Extent: vulkan.VkExtent2D{Width: maxDim, Height: maxDim}}}
	viewportInf := vulkan.VkPipelineViewportStateCreateInfo{
		SType:         vulkan.VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO,
		ViewportCount: 1,
		ScissorCount:  uint32(len(scissors)),
		PScissors:     unsafe.Pointer(&scissors[0]),
	}
	enable := vulkan.VkBool32(0)
	if desc.Blend {
		enable = 1
	}
	attBlendInf := vulkan.VkPipelineColorBlendAttachmentState{
		BlendEnable:         enable,
		SrcColorBlendFactor: desc.SrcFactor,
		SrcAlphaBlendFactor: desc.SrcFactor,
		DstColorBlendFactor: desc.DstFactor,
		DstAlphaBlendFactor: desc.DstFactor,
		ColorBlendOp:        vulkan.VK_BLEND_OP_ADD,
		AlphaBlendOp:        vulkan.VK_BLEND_OP_ADD,
		ColorWriteMask:      vulkan.VK_COLOR_COMPONENT_R_BIT | vulkan.VK_COLOR_COMPONENT_G_BIT | vulkan.VK_COLOR_COMPONENT_B_BIT | vulkan.VK_COLOR_COMPONENT_A_BIT,
	}
	blendInf := vulkan.VkPipelineColorBlendStateCreateInfo{
		SType:           vulkan.VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO,
		AttachmentCount: 1,
		PAttachments:    unsafe.Pointer(&attBlendInf),
	}
	var vkBinds []vulkan.VkVertexInputBindingDescription
	var vkAttrs []vulkan.VkVertexInputAttributeDescription
	for _, b := range desc.Bindings {
		vkBinds = append(vkBinds, vulkan.VkVertexInputBindingDescription{
			Binding: uint32(b.Binding),
			Stride:  uint32(b.Stride),
		})
	}
	for _, a := range desc.Attrs {
		vkAttrs = append(vkAttrs, vulkan.VkVertexInputAttributeDescription{
			Location: uint32(a.Location),
			Binding:  uint32(a.Binding),
			Format:   a.Format,
			Offset:   uint32(a.Offset),
		})
	}
	vertexInf := vulkan.VkPipelineVertexInputStateCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO,
	}
	if n := len(vkBinds); n > 0 {
		vertexInf.VertexBindingDescriptionCount = uint32(n)
		vertexInf.PVertexBindingDescriptions = unsafe.Pointer(&vkBinds[0])
	}
	if n := len(vkAttrs); n > 0 {
		vertexInf.VertexAttributeDescriptionCount = uint32(n)
		vertexInf.PVertexAttributeDescriptions = unsafe.Pointer(&vkAttrs[0])
	}
	inputAssemblyInf := vulkan.VkPipelineInputAssemblyStateCreateInfo{
		SType:    vulkan.VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO,
		Topology: desc.Topology,
	}
	rasterInf := vulkan.VkPipelineRasterizationStateCreateInfo{
		SType:     vulkan.VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO,
		CullMode:  desc.Cull,
		FrontFace: vulkan.VK_FRONT_FACE_CLOCKWISE,
		LineWidth: 1.0,
	}
	multiInf := vulkan.VkPipelineMultisampleStateCreateInfo{
		SType:                vulkan.VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO,
		RasterizationSamples: vulkan.VK_SAMPLE_COUNT_1_BIT,
	}
	inf := vulkan.VkGraphicsPipelineCreateInfo{
		SType:               vulkan.VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO,
		StageCount:          uint32(len(stages)),
		PStages:             unsafe.Pointer(&stages[0]),
		PVertexInputState:   unsafe.Pointer(&vertexInf),
		PInputAssemblyState: unsafe.Pointer(&inputAssemblyInf),
		PViewportState:      unsafe.Pointer(&viewportInf),
		PRasterizationState: unsafe.Pointer(&rasterInf),
		PMultisampleState:   unsafe.Pointer(&multiInf),
		PColorBlendState:    unsafe.Pointer(&blendInf),
		PDynamicState:       unsafe.Pointer(&dynInf),
		Layout:              desc.Layout,
		RenderPass:          pass,
	}
	var depthInf vulkan.VkPipelineDepthStencilStateCreateInfo
	if ds := desc.Depth; ds != nil {
		depthInf = vulkan.VkPipelineDepthStencilStateCreateInfo{
			SType:                 vulkan.VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO,
			DepthTestEnable:       vkBool(ds.Test),
			DepthWriteEnable:      vkBool(ds.Write),
			DepthCompareOp:        vulkan.VK_COMPARE_OP_LESS,
			DepthBoundsTestEnable: 0,
			StencilTestEnable:     0,
			MinDepthBounds:        0,
			MaxDepthBounds:        1,
		}
		inf.PDepthStencilState = unsafe.Pointer(&depthInf)
	}

	var pipe Pipeline
	// createInfoCount is the number of VkGraphicsPipelineCreateInfo in
	// pCreateInfos (one), not the number of shader stages: passing
	// len(stages) makes the driver read a second, garbage create info
	// struct right after inf and fault on its pointers.
	if err := vkErr(vulkan.VkCreateGraphicsPipelines(uintptr(d), 0, 1, unsafe.Pointer(&inf), nil, unsafe.Pointer(&pipe))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateGraphicsPipelines: %w", err)
	}
	return pipe, nil
}

func DestroyPipeline(d Device, p Pipeline) {
	vulkan.VkDestroyPipeline(uintptr(d), p, nil)
}

func CreatePipelineLayout(d Device, pushRanges []PushConstantRange, sets []DescriptorSetLayout) (PipelineLayout, error) {
	inf := vulkan.VkPipelineLayoutCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO,
	}
	if n := len(sets); n > 0 {
		inf.SetLayoutCount = uint32(n)
		inf.PSetLayouts = unsafe.Pointer(&sets[0])
	}
	if n := len(pushRanges); n > 0 {
		inf.PushConstantRangeCount = uint32(n)
		inf.PPushConstantRanges = unsafe.Pointer(&pushRanges[0])
	}
	var l PipelineLayout
	if err := vkErr(vulkan.VkCreatePipelineLayout(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&l))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreatePipelineLayout: %w", err)
	}
	return l, nil
}

func DestroyPipelineLayout(d Device, l PipelineLayout) {
	vulkan.VkDestroyPipelineLayout(uintptr(d), l, nil)
}

func CreateDescriptorSetLayout(d Device, bindings []DescriptorSetLayoutBinding) (DescriptorSetLayout, error) {
	var vkbinds []vulkan.VkDescriptorSetLayoutBinding
	for _, b := range bindings {
		vkbinds = append(vkbinds, vulkan.VkDescriptorSetLayoutBinding{
			Binding:         uint32(b.Binding),
			DescriptorType:  b.DescriptorType,
			DescriptorCount: 1,
			StageFlags:      b.StageFlags,
		})
	}
	inf := vulkan.VkDescriptorSetLayoutCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO,
	}
	if n := len(vkbinds); n > 0 {
		inf.BindingCount = uint32(n)
		inf.PBindings = unsafe.Pointer(&vkbinds[0])
	}
	var l DescriptorSetLayout
	if err := vkErr(vulkan.VkCreateDescriptorSetLayout(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&l))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateDescriptorSetLayout: %w", err)
	}
	return l, nil
}

func DestroyDescriptorSetLayout(d Device, l DescriptorSetLayout) {
	vulkan.VkDestroyDescriptorSetLayout(uintptr(d), l, nil)
}

func MapMemory(d Device, mem DeviceMemory, offset, size int) ([]byte, error) {
	var ptr unsafe.Pointer
	if err := vkErr(vulkan.VkMapMemory(uintptr(d), mem, DeviceSize(offset), DeviceSize(size), 0, unsafe.Pointer(&ptr))); err != nil {
		return nil, fmt.Errorf("vulkan: vkMapMemory: %w", err)
	}
	return unsafe.Slice((*byte)(ptr), size), nil
}

func UnmapMemory(d Device, mem DeviceMemory) {
	vulkan.VkUnmapMemory(uintptr(d), mem)
}

func ResetCommandBuffer(buf CommandBuffer) error {
	if err := vkErr(vulkan.VkResetCommandBuffer(uintptr(buf), 0)); err != nil {
		return fmt.Errorf("vulkan: vkResetCommandBuffer. %w", err)
	}
	return nil
}

func CreateDescriptorPool(d Device, maxSets int, sizes []DescriptorPoolSize) (DescriptorPool, error) {
	inf := vulkan.VkDescriptorPoolCreateInfo{
		SType:   vulkan.VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
		MaxSets: uint32(maxSets),
	}
	if n := len(sizes); n > 0 {
		inf.PoolSizeCount = uint32(n)
		inf.PPoolSizes = unsafe.Pointer(&sizes[0])
	}
	var pool DescriptorPool
	if err := vkErr(vulkan.VkCreateDescriptorPool(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&pool))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateDescriptorPool: %w", err)
	}
	return pool, nil
}

func DestroyDescriptorPool(d Device, pool DescriptorPool) {
	vulkan.VkDestroyDescriptorPool(uintptr(d), pool, nil)
}

func ResetDescriptorPool(d Device, pool DescriptorPool) error {
	if err := vkErr(vulkan.VkResetDescriptorPool(uintptr(d), pool, 0)); err != nil {
		return fmt.Errorf("vulkan: vkResetDescriptorPool: %w", err)
	}
	return nil
}

func UpdateDescriptorSet(d Device, write WriteDescriptorSet) {
	vulkan.VkUpdateDescriptorSets(uintptr(d), 1, unsafe.Pointer(&write), 0, nil)
}

func AllocateDescriptorSets(d Device, pool DescriptorPool, layout DescriptorSetLayout, count int) ([]DescriptorSet, error) {
	layouts := make([]DescriptorSetLayout, count)
	for i := range layouts {
		layouts[i] = layout
	}
	inf := vulkan.VkDescriptorSetAllocateInfo{
		SType:              vulkan.VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
		DescriptorPool:     pool,
		DescriptorSetCount: uint32(count),
		PSetLayouts:        unsafe.Pointer(&layouts[0]),
	}
	sets := make([]DescriptorSet, count)
	if err := vkErr(vulkan.VkAllocateDescriptorSets(uintptr(d), unsafe.Pointer(&inf), unsafe.Pointer(&sets[0]))); err != nil {
		return nil, fmt.Errorf("vulkan: vkAllocateDescriptorSets: %w", err)
	}
	return sets, nil
}

func CreateComputePipeline(d Device, mod ShaderModule, layout PipelineLayout) (Pipeline, error) {
	main := append([]byte("main"), 0)
	inf := vulkan.VkComputePipelineCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO,
		Stage: vulkan.VkPipelineShaderStageCreateInfo{
			SType:  vulkan.VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO,
			Stage:  vulkan.VK_SHADER_STAGE_COMPUTE_BIT,
			Module: mod,
			PName:  unsafe.Pointer(&main[0]),
		},
		Layout: layout,
	}
	var pipe Pipeline
	if err := vkErr(vulkan.VkCreateComputePipelines(uintptr(d), 0, 1, unsafe.Pointer(&inf), nil, unsafe.Pointer(&pipe))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateComputePipelines: %w", err)
	}
	return pipe, nil
}

func CreateFence(d Device, flags int) (Fence, error) {
	inf := vulkan.VkFenceCreateInfo{
		SType: vulkan.VK_STRUCTURE_TYPE_FENCE_CREATE_INFO,
		Flags: vulkan.VkFenceCreateFlags(flags),
	}
	var f Fence
	if err := vkErr(vulkan.VkCreateFence(uintptr(d), unsafe.Pointer(&inf), nil, unsafe.Pointer(&f))); err != nil {
		return 0, fmt.Errorf("vulkan: vkCreateFence: %w", err)
	}
	return f, nil
}

func DestroyFence(d Device, f Fence) {
	vulkan.VkDestroyFence(uintptr(d), f, nil)
}

func WaitForFences(d Device, fences ...Fence) error {
	if len(fences) == 0 {
		return nil
	}
	err := vkErr(vulkan.VkWaitForFences(uintptr(d), uint32(len(fences)), unsafe.Pointer(&fences[0]), 1, wholeSize))
	if err != nil {
		return fmt.Errorf("vulkan: vkWaitForFences: %w", err)
	}
	return nil
}

func ResetFences(d Device, fences ...Fence) error {
	if len(fences) == 0 {
		return nil
	}
	err := vkErr(vulkan.VkResetFences(uintptr(d), uint32(len(fences)), unsafe.Pointer(&fences[0])))
	if err != nil {
		return fmt.Errorf("vulkan: vkResetFences: %w", err)
	}
	return nil
}

func BuildSubpassDependency(srcStage, dstStage PipelineStageFlags, srcMask, dstMask AccessFlags, flags DependencyFlags) SubpassDependency {
	return SubpassDependency{
		SrcSubpass:      subpassExternal,
		SrcStageMask:    srcStage,
		SrcAccessMask:   srcMask,
		DstSubpass:      0,
		DstStageMask:    dstStage,
		DstAccessMask:   dstMask,
		DependencyFlags: flags,
	}
}

func BuildPushConstantRange(stages ShaderStageFlags, offset, size int) PushConstantRange {
	return PushConstantRange{
		StageFlags: stages,
		Offset:     uint32(offset),
		Size:       uint32(size),
	}
}

func BuildDescriptorPoolSize(typ DescriptorType, count int) DescriptorPoolSize {
	return DescriptorPoolSize{
		Type:            typ,
		DescriptorCount: uint32(count),
	}
}

func BuildWriteDescriptorSetImage(set DescriptorSet, binding int, typ DescriptorType, sampler Sampler, view ImageView, layout ImageLayout) WriteDescriptorSet {
	return WriteDescriptorSet{
		SType:           vulkan.VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,
		DstSet:          set,
		DstBinding:      uint32(binding),
		DescriptorCount: 1,
		DescriptorType:  typ,
		PImageInfo: unsafe.Pointer(&vulkan.VkDescriptorImageInfo{
			Sampler:     sampler,
			ImageView:   view,
			ImageLayout: layout,
		}),
	}
}

func BuildWriteDescriptorSetBuffer(set DescriptorSet, binding int, typ DescriptorType, buf Buffer) WriteDescriptorSet {
	return WriteDescriptorSet{
		SType:           vulkan.VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,
		DstSet:          set,
		DstBinding:      uint32(binding),
		DescriptorCount: 1,
		DescriptorType:  typ,
		PBufferInfo: unsafe.Pointer(&vulkan.VkDescriptorBufferInfo{
			Buffer: buf,
			Offset: 0,
			Range:  wholeSize,
		}),
	}
}

func PushConstantRangeStageFlags(r PushConstantRange) ShaderStageFlags {
	return r.StageFlags
}

func PushConstantRangeOffset(r PushConstantRange) int {
	return int(r.Offset)
}

func PushConstantRangeSize(r PushConstantRange) int {
	return int(r.Size)
}

func QueueFamilyPropertiesFlags(p QueueFamilyProperties) QueueFlags {
	return p.QueueFlags
}

func SurfaceCapabilitiesMinExtent(c SurfaceCapabilities) image.Point {
	return image.Pt(int(c.MinImageExtent.Width), int(c.MinImageExtent.Height))
}

func SurfaceCapabilitiesMaxExtent(c SurfaceCapabilities) image.Point {
	return image.Pt(int(c.MaxImageExtent.Width), int(c.MaxImageExtent.Height))
}

func BuildViewport(x, y, width, height float32) Viewport {
	return Viewport{
		X:        x,
		Y:        y,
		Width:    width,
		Height:   height,
		MaxDepth: 1.0,
	}
}

func BuildImageMemoryBarrier(img Image, srcMask, dstMask AccessFlags, oldLayout, newLayout ImageLayout, baseMip, numMips int) ImageMemoryBarrier {
	return ImageMemoryBarrier{
		SType:         vulkan.VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
		SrcAccessMask: srcMask,
		DstAccessMask: dstMask,
		OldLayout:     oldLayout,
		NewLayout:     newLayout,
		Image:         img,
		SubresourceRange: vulkan.VkImageSubresourceRange{
			AspectMask:   vulkan.VK_IMAGE_ASPECT_COLOR_BIT,
			BaseMipLevel: uint32(baseMip),
			LevelCount:   uint32(numMips),
			LayerCount:   remainingArrayLayers,
		},
	}
}

func BuildBufferMemoryBarrier(buf Buffer, srcMask, dstMask AccessFlags) BufferMemoryBarrier {
	return BufferMemoryBarrier{
		SType:         vulkan.VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER,
		SrcAccessMask: srcMask,
		DstAccessMask: dstMask,
		Buffer:        buf,
		Size:          wholeSize,
	}
}

func BuildMemoryBarrier(srcMask, dstMask AccessFlags) MemoryBarrier {
	return MemoryBarrier{
		SType:         vulkan.VK_STRUCTURE_TYPE_MEMORY_BARRIER,
		SrcAccessMask: srcMask,
		DstAccessMask: dstMask,
	}
}

func BuildBufferImageCopy(bufOff, bufStride, x, y, width, height int) BufferImageCopy {
	return BuildBufferImageCopyMip(bufOff, bufStride, x, y, width, height, 0)
}

// BuildBufferImageCopyMip is BuildBufferImageCopy with the mip level named:
// without it every copy lands on level zero, and a chain uploaded level by
// level overwrites the same level and leaves the rest uninitialised.
func BuildBufferImageCopyMip(bufOff, bufStride, x, y, width, height, mip int) BufferImageCopy {
	return BufferImageCopy{
		BufferOffset:    DeviceSize(bufOff),
		BufferRowLength: uint32(bufStride),
		ImageSubresource: vulkan.VkImageSubresourceLayers{
			AspectMask: vulkan.VK_IMAGE_ASPECT_COLOR_BIT,
			MipLevel:   uint32(mip),
			LayerCount: 1,
		},
		ImageOffset: vulkan.VkOffset3D{
			X: int32(x), Y: int32(y), Z: 0,
		},
		ImageExtent: vulkan.VkExtent3D{
			Width: uint32(width), Height: uint32(height), Depth: 1,
		},
	}
}

func BuildImageCopy(srcX, srcY, dstX, dstY, width, height int) ImageCopy {
	return ImageCopy{
		SrcSubresource: vulkan.VkImageSubresourceLayers{
			AspectMask: vulkan.VK_IMAGE_ASPECT_COLOR_BIT,
			LayerCount: 1,
		},
		SrcOffset: vulkan.VkOffset3D{
			X: int32(srcX),
			Y: int32(srcY),
		},
		DstSubresource: vulkan.VkImageSubresourceLayers{
			AspectMask: vulkan.VK_IMAGE_ASPECT_COLOR_BIT,
			LayerCount: 1,
		},
		DstOffset: vulkan.VkOffset3D{
			X: int32(dstX),
			Y: int32(dstY),
		},
		Extent: vulkan.VkExtent3D{
			Width:  uint32(width),
			Height: uint32(height),
			Depth:  1,
		},
	}
}

func BuildImageBlit(srcX, srcY, dstX, dstY, srcWidth, srcHeight, dstWidth, dstHeight, srcMip, dstMip int) ImageBlit {
	return ImageBlit{
		SrcOffsets: [2]vulkan.VkOffset3D{
			{X: int32(srcX), Y: int32(srcY), Z: 0},
			{X: int32(srcWidth), Y: int32(srcHeight), Z: 1},
		},
		SrcSubresource: vulkan.VkImageSubresourceLayers{
			AspectMask: vulkan.VK_IMAGE_ASPECT_COLOR_BIT,
			LayerCount: 1,
			MipLevel:   uint32(srcMip),
		},
		DstOffsets: [2]vulkan.VkOffset3D{
			{X: int32(dstX), Y: int32(dstY), Z: 0},
			{X: int32(dstWidth), Y: int32(dstHeight), Z: 1},
		},
		DstSubresource: vulkan.VkImageSubresourceLayers{
			AspectMask: vulkan.VK_IMAGE_ASPECT_COLOR_BIT,
			LayerCount: 1,
			MipLevel:   uint32(dstMip),
		},
	}
}

func findMemoryTypeIndex(pd PhysicalDevice, constraints uint32, wantProps MemoryPropertyFlags) (int, bool) {
	var memProps vulkan.VkPhysicalDeviceMemoryProperties
	vulkan.VkGetPhysicalDeviceMemoryProperties(uintptr(pd), unsafe.Pointer(&memProps))

	for i := 0; i < int(memProps.MemoryTypeCount); i++ {
		if (constraints & (1 << i)) == 0 {
			continue
		}
		if (memProps.MemoryTypes[i].PropertyFlags & wantProps) != wantProps {
			continue
		}
		return i, true
	}

	return 0, false
}

func choosePresentMode(pd PhysicalDevice, surf Surface) (vulkan.VkPresentModeKHR, bool, error) {
	var count uint32
	err := vkErr(vulkan.VkGetPhysicalDeviceSurfacePresentModesKHR(uintptr(pd), surf, unsafe.Pointer(&count), nil))
	if err != nil {
		return 0, false, fmt.Errorf("vulkan: vkGetPhysicalDeviceSurfacePresentModesKHR: %w", err)
	}
	if count == 0 {
		return 0, false, nil
	}
	modes := make([]vulkan.VkPresentModeKHR, count)
	err = vkErr(vulkan.VkGetPhysicalDeviceSurfacePresentModesKHR(uintptr(pd), surf, unsafe.Pointer(&count), unsafe.Pointer(&modes[0])))
	if err != nil {
		return 0, false, fmt.Errorf("vulkan: kGetPhysicalDeviceSurfacePresentModesKHR: %w", err)
	}
	for _, m := range modes {
		if m == vulkan.VK_PRESENT_MODE_MAILBOX_KHR || m == vulkan.VK_PRESENT_MODE_FIFO_KHR {
			return m, true, nil
		}
	}
	return 0, false, nil
}

func chooseFormat(pd PhysicalDevice, surf Surface) (vulkan.VkSurfaceFormatKHR, bool, error) {
	var count uint32
	err := vkErr(vulkan.VkGetPhysicalDeviceSurfaceFormatsKHR(uintptr(pd), surf, unsafe.Pointer(&count), nil))
	if err != nil {
		return vulkan.VkSurfaceFormatKHR{}, false, fmt.Errorf("vulkan: vkGetPhysicalDeviceSurfaceFormatsKHR: %w", err)
	}
	if count == 0 {
		return vulkan.VkSurfaceFormatKHR{}, false, nil
	}
	formats := make([]vulkan.VkSurfaceFormatKHR, count)
	err = vkErr(vulkan.VkGetPhysicalDeviceSurfaceFormatsKHR(uintptr(pd), surf, unsafe.Pointer(&count), unsafe.Pointer(&formats[0])))
	if err != nil {
		return vulkan.VkSurfaceFormatKHR{}, false, fmt.Errorf("vulkan: vkGetPhysicalDeviceSurfaceFormatsKHR: %w", err)
	}
	// Query for format with sRGB support.
	// TODO: Support devices without sRGB.
	for _, f := range formats {
		if f.ColorSpace != vulkan.VK_COLOR_SPACE_SRGB_NONLINEAR_KHR {
			continue
		}
		switch f.Format {
		case vulkan.VK_FORMAT_B8G8R8A8_SRGB, vulkan.VK_FORMAT_R8G8B8A8_SRGB:
			return f, true, nil
		}
	}
	return vulkan.VkSurfaceFormatKHR{}, false, nil
}

func chooseQueue(pd PhysicalDevice, surf Surface, flags QueueFlags) (int, bool, error) {
	queues := GetPhysicalDeviceQueueFamilyProperties(pd)
	for i, q := range queues {
		// Check for presentation and feature support.
		if q.QueueFlags&flags != flags {
			continue
		}
		if surf != 0 {
			// Check for presentation support. It is possible that a device has no
			// queue with both rendering and presentation support, but not in reality.
			// See https://github.com/KhronosGroup/Vulkan-Docs/issues/1234.
			var support vulkan.VkBool32
			if err := vkErr(vulkan.VkGetPhysicalDeviceSurfaceSupportKHR(uintptr(pd), uint32(i), surf, unsafe.Pointer(&support))); err != nil {
				return 0, false, fmt.Errorf("vulkan: vkGetPhysicalDeviceSurfaceSupportKHR: %w", err)
			}
			if support != 1 {
				continue
			}
		}
		return i, true, nil
	}
	return 0, false, nil
}

func vkErr(res vulkan.VkResult) error {
	switch res {
	case vulkan.VK_SUCCESS:
		return nil
	default:
		return Error(res)
	}
}

func (e Error) Error() string {
	return fmt.Sprintf("error %d", int32(e))
}

// Formats, layouts, flags and stages that custom passes need and Gio's own
// rendering paths never touch.
const (
	FORMAT_D32_SFLOAT    Format = vulkan.VK_FORMAT_D32_SFLOAT
	FORMAT_BC1_RGBA_SRGB Format = vulkan.VK_FORMAT_BC1_RGBA_SRGB_BLOCK
	FORMAT_BC2_SRGB      Format = vulkan.VK_FORMAT_BC2_SRGB_BLOCK
	FORMAT_BC3_SRGB      Format = vulkan.VK_FORMAT_BC3_SRGB_BLOCK

	FORMAT_FEATURE_DEPTH_STENCIL_ATTACHMENT_BIT FormatFeatureFlags = vulkan.VK_FORMAT_FEATURE_DEPTH_STENCIL_ATTACHMENT_BIT

	IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT ImageUsageFlags = vulkan.VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT

	IMAGE_ASPECT_COLOR_BIT uint32 = 0x1
	IMAGE_ASPECT_DEPTH_BIT uint32 = 0x2

	ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT  AccessFlags = vulkan.VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT
	ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT AccessFlags = vulkan.VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT

	PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT
	PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT  PipelineStageFlags = vulkan.VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT

	IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL ImageLayout = vulkan.VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL
)

// CullMode selects which triangle windings are discarded. The values are the
// VkCullModeFlagBits numbers.
type CullMode = vulkan.VkCullModeFlags

const (
	CULL_MODE_NONE  CullMode = vulkan.VK_CULL_MODE_NONE
	CULL_MODE_FRONT CullMode = vulkan.VK_CULL_MODE_FRONT_BIT
	CULL_MODE_BACK  CullMode = vulkan.VK_CULL_MODE_BACK_BIT
)

// vkBool converts a Go bool to the VkBool32 the C structs use.
func vkBool(b bool) vulkan.VkBool32 {
	if b {
		return 1
	}
	return 0
}

// BuildImageMemoryBarrierEx is BuildImageMemoryBarrier for an arbitrary image
// aspect, which is what a depth buffer needs: its barrier must name the depth
// aspect, not the color one.
func BuildImageMemoryBarrierEx(img Image, srcMask, dstMask AccessFlags, oldLayout, newLayout ImageLayout, baseMip, numMips int, aspect uint32) ImageMemoryBarrier {
	b := BuildImageMemoryBarrier(img, srcMask, dstMask, oldLayout, newLayout, baseMip, numMips)
	b.SubresourceRange.AspectMask = aspect
	return b
}
