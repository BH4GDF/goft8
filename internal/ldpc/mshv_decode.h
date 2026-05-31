#ifndef MSHV_DECODE_H
#define MSHV_DECODE_H

#ifdef __cplusplus
extern "C" {
#endif

/* C-compatible wrapper for MSHV's decode174_91.
 *
 * llr:      174 LLR values (double)
 * maxosd:   max OSD passes (-1=BP only, 0=BP+1 OSD, >0=BP+maxosd OSD)
 * norder:   OSD search depth (ndeep)
 * apmask:   174 int values (0/1), pinned bits
 * message91: output 91-bit message (0/1)
 * cw:       output 174-bit codeword (0/1)
 * nharderror: output number of hard errors (negative = decode failed)
 * dmin:     output minimum distance metric
 */
void mshv_decode174_91(double *llr, int maxosd, int norder, int *apmask,
                       int *message91, int *cw, int *nharderror, double *dmin);

#ifdef __cplusplus
}
#endif

#endif
