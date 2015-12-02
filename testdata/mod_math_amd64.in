/*
Quicker way of loading MOD_P ?
        MOVL    $-1, R8
        NEGQ    R8
*/

#define MOD_P 0xFFFFFFFF00000001

// Compute x = x - y mod p
// Preserves y, p
#define MOD_SUB(x, y, p, LABEL) \
	SUBQ    y, x; \
	JCC     LABEL; \
	ADDQ    p, x; \
LABEL:	\

// Compute x = x - y mod p
// Preserves y, p
#define MOD_SUB2(x, y, p, LABEL) \
        XORQ R10,R10; \
	SUBQ    y, x; \
        CMOVQCS  p, R9; \
        CMOVQCC  R10, R9; \
        ADDQ    R9, x; \

// Compute x = x + y mod p, using t
// Preserves y, p
#define MOD_ADD(x, y, t, p, LABEL) \
        MOVQ    p, t; \
	SUBQ    y, t; \
        MOD_SUB(x, t, p, LABEL); \

TEXT ·mod_sub(SB),7,$0-24
	MOVQ    $MOD_P,R8
	MOVQ    x+0(FP),AX
	MOVQ    y+8(FP),CX
        MOD_SUB(AX, CX, R8, sub1)
	MOVQ    AX,ret+16(FP)
	RET

TEXT ·mod_add(SB),7,$0-24
	MOVQ    $MOD_P,R8
	MOVQ    x+0(FP),AX
	MOVQ    y+8(FP),BP
        MOD_ADD(AX, BP, CX, R8, add1)
	MOVQ    AX,ret+16(FP)
	RET

// Reduce 128 bits mod p (b, a) -> a
// Using t0, t1
// This is much faster (2 or 3 times) than DIVQ
#define MOD_REDUCE(b, a, t0, t1, p, label) \
        MOVL    b, t0;	/* Also sets upper 32 bits to 0 */ \
	SHRQ    $32,b ; \
 ; \
	CMPQ    a,p ; \
	JCS     label/**/1 ; \
	SUBQ    p,a ; \
label/**/1: ; \
 ; \
	MOVLQZX t0,t1 ; \
        MOD_SUB(a, t1, p, label/**/2) ; \
 ; \
	MOVLQZX b,t1 ; \
        MOD_SUB(a, t1, p, label/**/3) ; \
 ; \
	SHLQ    $32,t0 ; \
        MOD_ADD(a, t0, t1, p, label/**/4) ; \

TEXT ·mod_reduce(SB),7,$0-24
	MOVQ    $MOD_P,R8
	MOVQ    b+0(FP),DI
	MOVQ    a+8(FP),AX

        MOD_REDUCE(DI, AX, SI, BX, R8, mod_reduce)

	MOVQ    AX,ret+16(FP)
	RET 

TEXT ·mod_mul(SB),7,$0-24
	MOVQ    x+0(FP),BX
	MOVQ    y+8(FP),AX
	MOVQ    $MOD_P,R8
        MULQ	BX /* BX * AX -> (DX, AX) */
        MOD_REDUCE(DX, AX, SI, BX, R8, mod_mul)
	MOVQ    AX,ret+16(FP)
	RET 

TEXT ·mod_sqr(SB),7,$0-16
	MOVQ    x+0(FP),AX
	MOVQ    $MOD_P,R8
        MULQ	AX /* AX * AX -> (DX, AX) */
        MOD_REDUCE(DX, AX, SI, BX, R8, mod_sqr)
	MOVQ    AX,ret+8(FP)
	RET 

#define MOD_SHIFT_0_TO_31(x, shift, t0, t1, p, label) \
        MOVQ     x, t0; \
	/* xmid_xlow := x << shift, (xmid, xlow) */ \
        SHLQ    $shift, x; \
	/* xhigh := uint32(x >> (64 - shift)) */ \
        SHRQ    $(64-shift), t0 ; \
	/* t := uint64(0xFFFFFFFF-xhigh)<<32 + uint64(xhigh+1), (2^32 - 1 - xhigh, xhigh + 1) */ \
        MOVL     t0, t1; \
        SHLQ    $32, t0; \
        SUBQ     t1, t0; \
	/* r := xmid_xlow - t, (xmid, xlow) + (xhigh, -xhigh) */ \
        MOD_ADD(x, t0, t1, p, label); \

#define MOD_SHIFT_0_TO_31_PROC(shift) \
	MOVQ    x+0(FP),AX; \
	MOVQ    $MOD_P,R8; \
        MOD_SHIFT_0_TO_31(AX, shift, BX, CX, R8, mod_shift/**/shift/**/a); \
	MOVQ    AX,ret+8(FP); \
	RET ; \

TEXT ·mod_shift3(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(3)
TEXT ·mod_shift6(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(6)
TEXT ·mod_shift9(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(9)
TEXT ·mod_shift12(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(12)
TEXT ·mod_shift15(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(15)
TEXT ·mod_shift18(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(18)
TEXT ·mod_shift21(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(21)
TEXT ·mod_shift24(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(24)
TEXT ·mod_shift27(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(27)
TEXT ·mod_shift30(SB),7,$0-16
	MOD_SHIFT_0_TO_31_PROC(30)

#define MOD_SHIFT_32_TO_63(x, shift, t0, t1, t2, p, label) \
        MOVQ x, t0; \
	/* xmid := uint32(x >> (64 - shift)) */ \
	/* xlow := uint32(x << (shift - 32)) */ \
        SHLQ $(shift-32),x; \
	/* xhigh := uint32(x >> (96 - shift)) */ \
        SHRQ $(96-shift),t0; \
        MOVL x, t1; /* xlow */ \
        SHRQ $32, x; /* xmid */ \
        SHLQ $32, t1; \
        MOVL x, t2; /* t1 */ \
	/* t0 := uint64(xmid) << 32 // (xmid, 0) */ \
        SHLQ $32, x; /* t0 */ \
	/* t1 := uint64(xmid), (0, xmid) */ \
	/* t0 -= t1, (xmid, -xmid) no carry and must be in range 0..p-1 */ \
        SUBQ t2, x; \
	/* t1 = uint64(xhigh), (0, xhigh) */ \
	/* r := t0 - t1, (xmid, - xhigh - xmid) */ \
        MOD_SUB(x, t0, p, label/**/1); \
	/* add (xlow, 0) */ \
        MOD_ADD(x, t1, t0, p, label/**/2); \

#define MOD_SHIFT_32_TO_63_PROC(shift) \
	MOVQ    x+0(FP),AX; \
	MOVQ    $MOD_P,R8; \
        MOD_SHIFT_32_TO_63(AX, shift, BX, CX, DX, R8, mod_shift/**/shift/**/a); \
	MOVQ    AX,ret+8(FP); \
	RET ; \

TEXT ·mod_shift33(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(33)
TEXT ·mod_shift36(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(36)
TEXT ·mod_shift39(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(39)
TEXT ·mod_shift42(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(42)
TEXT ·mod_shift45(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(45)
TEXT ·mod_shift48(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(48)
TEXT ·mod_shift51(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(51)
TEXT ·mod_shift54(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(54)
TEXT ·mod_shift57(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(57)
TEXT ·mod_shift60(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(60)
TEXT ·mod_shift63(SB),7,$0-16
	MOD_SHIFT_32_TO_63_PROC(63)

#define MOD_SHIFT_64_TO_95(x, shift, t0, t1, p, label) \
        MOVQ x, t0; \
	/* xlow := uint32(x << (shift - 64)) */ \
        SHLL $(shift - 64), x; \
	/* xhigh := uint32(x >> (128 - shift)) */ \
	/* xmid := uint32(x >> (96 - shift)) */ \
        SHRQ $(96 - shift), t0; \
	/* t0 := uint64(xlow) << 32, (xlow, 0) */ \
        MOVL x, t1; \
        SHLQ $32, x; \
	/* t1 := uint64(xlow), (0, xlow) */ \
	/* t0 -= t1, (xlow, -xlow) - no carry possible */ \
        SUBQ t1, x; \
	/* t1 = uint64(xhigh)<<32 + uint64(xmid), (xhigh, xmid) */ \
	/* r := t0 - t1, (xlow, -xlow) - (xhigh, xmid) */ \
        MOD_SUB(x, t0, p, label); \

#define MOD_SHIFT_64_TO_95_PROC(shift) \
	MOVQ    x+0(FP),AX; \
	MOVQ    $MOD_P,R8; \
        MOD_SHIFT_64_TO_95(AX, shift, BX, CX, R8, mod_shift/**/shift/**/a); \
	MOVQ    AX,ret+8(FP); \
	RET ; \

TEXT ·mod_shift66(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(66)
TEXT ·mod_shift69(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(69)
TEXT ·mod_shift72(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(72)
TEXT ·mod_shift75(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(75)
TEXT ·mod_shift78(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(78)
TEXT ·mod_shift81(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(81)
TEXT ·mod_shift84(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(84)
TEXT ·mod_shift87(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(87)
TEXT ·mod_shift90(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(90)
TEXT ·mod_shift93(SB),7,$0-16
	MOD_SHIFT_64_TO_95_PROC(93)

#include "ffts_amd64.h"
