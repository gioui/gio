// SPDX-License-Identifier: Unlicense OR MIT

/*
Package clip provides operations for clipping paint operations.
Drawing outside the current clip area is ignored.

The current clip is initially the infinite set. Pushing and Op sets the clip
to the intersection of the current clip and pushed clip area. Popping the
area restores the clip to its state before pushing.

General clipping areas are constructed with Path. Common cases such as
rectangular clip areas also exist as convenient constructors.
*/
package clip
