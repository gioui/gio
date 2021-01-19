// SPDX-License-Identifier: Unlicense OR MIT

#include "textflag.h"

TEXT ·newMethod(SB), NOSPLIT, $0
  CallImport
  RET

TEXT ·call(SB), NOSPLIT, $0
  CallImport
  RET

TEXT ·get(SB), NOSPLIT, $0
  CallImport
  RET

TEXT ·free(SB), NOSPLIT, $0
  CallImport
  RET

TEXT ·buffer(SB), NOSPLIT, $0
  CallImport
  RET
