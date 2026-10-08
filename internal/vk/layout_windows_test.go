//go:build windows

package vk

import (
	"testing"
	"unsafe"

	"github.com/christerso/vulkan-go/vulkan"
)

// TestStructLayouts verifies that the cgo-free vulkan-go struct definitions
// match the canonical C layouts from vulkan.h on 64-bit Windows. A mismatch
// here makes the driver read garbage pointers inside vkCreate* calls.
func TestStructLayouts(t *testing.T) {
	checkSize := func(name string, got, want uintptr) {
		t.Helper()
		if got != want {
			t.Errorf("sizeof %s = %d, want %d", name, got, want)
		}
	}
	checkOffset := func(name string, got, want uintptr) {
		t.Helper()
		if got != want {
			t.Errorf("offsetof %s = %d, want %d", name, got, want)
		}
	}

	checkSize("VkPipelineShaderStageCreateInfo", unsafe.Sizeof(vulkan.VkPipelineShaderStageCreateInfo{}), 48)
	checkSize("VkPipelineVertexInputStateCreateInfo", unsafe.Sizeof(vulkan.VkPipelineVertexInputStateCreateInfo{}), 48)
	checkSize("VkPipelineInputAssemblyStateCreateInfo", unsafe.Sizeof(vulkan.VkPipelineInputAssemblyStateCreateInfo{}), 32)
	checkSize("VkPipelineViewportStateCreateInfo", unsafe.Sizeof(vulkan.VkPipelineViewportStateCreateInfo{}), 48)
	checkSize("VkPipelineRasterizationStateCreateInfo", unsafe.Sizeof(vulkan.VkPipelineRasterizationStateCreateInfo{}), 64)
	checkSize("VkPipelineMultisampleStateCreateInfo", unsafe.Sizeof(vulkan.VkPipelineMultisampleStateCreateInfo{}), 48)
	checkSize("VkPipelineColorBlendAttachmentState", unsafe.Sizeof(vulkan.VkPipelineColorBlendAttachmentState{}), 32)
	checkSize("VkPipelineColorBlendStateCreateInfo", unsafe.Sizeof(vulkan.VkPipelineColorBlendStateCreateInfo{}), 56)
	checkSize("VkPipelineDynamicStateCreateInfo", unsafe.Sizeof(vulkan.VkPipelineDynamicStateCreateInfo{}), 32)
	checkSize("VkGraphicsPipelineCreateInfo", unsafe.Sizeof(vulkan.VkGraphicsPipelineCreateInfo{}), 144)
	checkSize("VkVertexInputBindingDescription", unsafe.Sizeof(vulkan.VkVertexInputBindingDescription{}), 12)
	checkSize("VkVertexInputAttributeDescription", unsafe.Sizeof(vulkan.VkVertexInputAttributeDescription{}), 16)
	checkSize("VkRect2D", unsafe.Sizeof(vulkan.VkRect2D{}), 16)
	checkSize("VkExtent2D", unsafe.Sizeof(vulkan.VkExtent2D{}), 8)
	checkSize("VkViewport", unsafe.Sizeof(vulkan.VkViewport{}), 24)

	// Field offsets inside VkGraphicsPipelineCreateInfo (C layout).
	var g vulkan.VkGraphicsPipelineCreateInfo
	checkOffset("g.PNext", unsafe.Offsetof(g.PNext), 8)
	checkOffset("g.Flags", unsafe.Offsetof(g.Flags), 16)
	checkOffset("g.StageCount", unsafe.Offsetof(g.StageCount), 20)
	checkOffset("g.PStages", unsafe.Offsetof(g.PStages), 24)
	checkOffset("g.PVertexInputState", unsafe.Offsetof(g.PVertexInputState), 32)
	checkOffset("g.PInputAssemblyState", unsafe.Offsetof(g.PInputAssemblyState), 40)
	checkOffset("g.PViewportState", unsafe.Offsetof(g.PViewportState), 56)
	checkOffset("g.PRasterizationState", unsafe.Offsetof(g.PRasterizationState), 64)
	checkOffset("g.PMultisampleState", unsafe.Offsetof(g.PMultisampleState), 72)
	checkOffset("g.PDepthStencilState", unsafe.Offsetof(g.PDepthStencilState), 80)
	checkOffset("g.PColorBlendState", unsafe.Offsetof(g.PColorBlendState), 88)
	checkOffset("g.PDynamicState", unsafe.Offsetof(g.PDynamicState), 96)
	checkOffset("g.Layout", unsafe.Offsetof(g.Layout), 104)
	checkOffset("g.RenderPass", unsafe.Offsetof(g.RenderPass), 112)
	checkOffset("g.Subpass", unsafe.Offsetof(g.Subpass), 120)
	checkOffset("g.BasePipelineHandle", unsafe.Offsetof(g.BasePipelineHandle), 128)
	checkOffset("g.BasePipelineIndex", unsafe.Offsetof(g.BasePipelineIndex), 136)

	// Shader stage: module@24, pName@32, pSpecializationInfo@40.
	var s vulkan.VkPipelineShaderStageCreateInfo
	checkOffset("s.Flags", unsafe.Offsetof(s.Flags), 16)
	checkOffset("s.Stage", unsafe.Offsetof(s.Stage), 20)
	checkOffset("s.Module", unsafe.Offsetof(s.Module), 24)
	checkOffset("s.PName", unsafe.Offsetof(s.PName), 32)
	checkOffset("s.PSpecializationInfo", unsafe.Offsetof(s.PSpecializationInfo), 40)

	// Render pass and shader module (used before pipeline creation).
	checkSize("VkRenderPassCreateInfo", unsafe.Sizeof(vulkan.VkRenderPassCreateInfo{}), 64)
	checkSize("VkAttachmentDescription", unsafe.Sizeof(vulkan.VkAttachmentDescription{}), 36)
	checkSize("VkAttachmentReference", unsafe.Sizeof(vulkan.VkAttachmentReference{}), 8)
	checkSize("VkSubpassDescription", unsafe.Sizeof(vulkan.VkSubpassDescription{}), 72)
	checkSize("VkShaderModuleCreateInfo", unsafe.Sizeof(vulkan.VkShaderModuleCreateInfo{}), 40)
	var sm vulkan.VkShaderModuleCreateInfo
	checkOffset("sm.CodeSize", unsafe.Offsetof(sm.CodeSize), 24)
	checkOffset("sm.PCode", unsafe.Offsetof(sm.PCode), 32)

	// Swapchain path (already exercised, but verify for completeness).
	checkSize("VkSurfaceCapabilitiesKHR", unsafe.Sizeof(vulkan.VkSurfaceCapabilitiesKHR{}), 52)
	checkSize("VkSwapchainCreateInfoKHR", unsafe.Sizeof(vulkan.VkSwapchainCreateInfoKHR{}), 104)
	var sc vulkan.VkSwapchainCreateInfoKHR
	// The C struct carries a surface handle between flags and minImageCount,
	// which is what puts minImageCount at 32 rather than 20.
	checkOffset("sc.MinImageCount", unsafe.Offsetof(sc.MinImageCount), 32)
	checkOffset("sc.ImageExtent", unsafe.Offsetof(sc.ImageExtent), 44)
	checkOffset("sc.OldSwapchain", unsafe.Offsetof(sc.OldSwapchain), 96)

	// Pipeline layout.
	checkSize("VkPipelineLayoutCreateInfo", unsafe.Sizeof(vulkan.VkPipelineLayoutCreateInfo{}), 48)
	var pl vulkan.VkPipelineLayoutCreateInfo
	checkOffset("pl.SetLayoutCount", unsafe.Offsetof(pl.SetLayoutCount), 20)
	checkOffset("pl.PSetLayouts", unsafe.Offsetof(pl.PSetLayouts), 24)
	checkOffset("pl.PushConstantRangeCount", unsafe.Offsetof(pl.PushConstantRangeCount), 32)
	checkOffset("pl.PPushConstantRanges", unsafe.Offsetof(pl.PPushConstantRanges), 40)
}
