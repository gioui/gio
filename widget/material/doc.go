// SPDX-License-Identifier: Unlicense OR MIT

// Package material implements the Material design.
//
// To maximize reusability and visual flexibility, user interface controls are
// split into two parts: the stateful widget and the stateless drawing of it.
//
// For example, widget.Button encapsulates the state and event
// handling of all buttons, while the Theme can draw a single Button
// in various styles.
//
// This snippet defines a button that prints a message when clicked:
//
//     var gtx *layout.Context
//     button := new(widget.Button)
//
//     for button.Clicked(gtx) {
//         fmt.Println("Clicked!")
//     }
//
// Use a Theme to draw the button:
//
//     theme := material.NewTheme(...)
//
//     th.Button("Click me!").Layout(gtx, button)
//
// Customization
//
// Quite often, a program needs to customize the theme provided defaults. Several
// options are available, depending on the nature of the change:
//
// Mandatory parameters: Some parameters are not part of the widget state but
// have no obvious default. In the program above, the button text is a
// parameter to the Theme.Button method.
//
// Theme-global parameters: For changing the look of all widgets drawn with a
// particular theme, adjust the `Theme` fields:
//
//     theme.Color.Primary = color.RGBA{...}
//
// Widget-local parameters: For changing the look of a particular widget,
// adjust the widget specific theme object:
//
//     btn := th.Button("Click me!")
//     btn.Font.Style = text.Italic
//     btn.Layout(gtx)
//
// Widget variants: A widget can have several distinct representations even
// though the underlying state is the same. A widget.Button can be drawn as a
// round icon button:
//
//     icon := material.NewIcon(...)
//
//     th.IconButton(icon).Layout(gtx, button)
//
// Specialized widgets: Theme both define a generic Label method
// that takes a text size, and specialized methods for standard text
// sizes such as Theme.H1 and Theme.Body2.
package material
