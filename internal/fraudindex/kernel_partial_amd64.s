// Copyright 2026.

#include "textflag.h"

// func BlockSquaredDistancePartial(query *[112]int16, block *[112]int16,
//                                  dimStart int, dimCount int,
//                                  accum *[8]uint64)
//
// Same per-lane squared L2 kernel as BlockSquaredDistance, but processes
// only dims [dimStart, dimStart+dimCount) and accumulates into the
// caller-supplied accum (read at entry, written at exit). Lets the caller
// split a 14-dim distance into chunks and bail out early when partial
// distances already exceed the current top-5 worst.
//
// Lane layout in accum is identical to the full kernel: lane 0..1 in the
// first xmm word, 2..3 in the next, and so on. Caller is responsible for
// zeroing the accum before the first chunk.
TEXT ·BlockSquaredDistancePartial(SB), NOSPLIT, $0-40
	MOVQ query+0(FP), AX
	MOVQ block+8(FP), BX
	MOVQ dimStart+16(FP), R8
	MOVQ dimCount+24(FP), DX
	MOVQ accum+32(FP), CX

	// Skip to dimStart: each dim is 8 int16 = 16 bytes.
	SHLQ $4, R8
	ADDQ R8, AX
	ADDQ R8, BX

	// Load accum into the four lane accumulators.
	MOVOU 0(CX), X10
	MOVOU 16(CX), X11
	MOVOU 32(CX), X12
	MOVOU 48(CX), X13

	TESTQ DX, DX
	JZ done

loop:
	MOVOU (AX), X0
	MOVOU (BX), X1

	// Low 4 lanes.
	PMOVSXWD X0, X2
	PMOVSXWD X1, X3
	PSUBL X3, X2
	PMULLD X2, X2

	PMOVZXDQ X2, X4
	PSRLO $8, X2
	PMOVZXDQ X2, X5
	PADDQ X4, X10
	PADDQ X5, X11

	// High 4 lanes.
	PSRLO $8, X0
	PSRLO $8, X1
	PMOVSXWD X0, X6
	PMOVSXWD X1, X7
	PSUBL X7, X6
	PMULLD X6, X6

	PMOVZXDQ X6, X8
	PSRLO $8, X6
	PMOVZXDQ X6, X9
	PADDQ X8, X12
	PADDQ X9, X13

	ADDQ $16, AX
	ADDQ $16, BX

	DECQ DX
	JNZ loop

done:
	MOVOU X10, 0(CX)
	MOVOU X11, 16(CX)
	MOVOU X12, 32(CX)
	MOVOU X13, 48(CX)
	RET
