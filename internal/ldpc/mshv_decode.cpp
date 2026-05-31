/* Extracted from MSHV decoderpom.cpp / decoderpom.h
 * Copyright 2015 Hrisimir Hristov, LZ2HV
 * May be used under the terms of the GNU General Public License (GPL)
 *
 * Standalone C++ LDPC/OSD decoder for FT8 (174,91).
 * All Qt dependencies removed.
 */

#include "mshv_decode.h"
#include <math.h>
#include <string.h>
#include <stdlib.h>
#include <stdbool.h>

/* ------------------------------------------------------------------ */
/* H-matrix definitions (from bpdecode_ft8_174_91.h)                  */
/* ------------------------------------------------------------------ */

static const char *g_ft8_174_91[83] =
{
    "8329ce11bf31eaf509f27fc",
    "761c264e25c259335493132",
    "dc265902fb277c6410a1bdc",
    "1b3f417858cd2dd33ec7f62",
    "09fda4fee04195fd034783a",
    "077cccc11b8873ed5c3d48a",
    "29b62afe3ca036f4fe1a9da",
    "6054faf5f35d96d3b0c8c3e",
    "e20798e4310eed27884ae90",
    "775c9c08e80e26ddae56318",
    "b0b811028c2bf997213487c",
    "18a0c9231fc60adf5c5ea32",
    "76471e8302a0721e01b12b8",
    "ffbccb80ca8341fafb47b2e",
    "66a72a158f9325a2bf67170",
    "c4243689fe85b1c51363a18",
    "0dff739414d1a1b34b1c270",
    "15b48830636c8b99894972e",
    "29a89c0d3de81d665489b0e",
    "4f126f37fa51cbe61bd6b94",
    "99c47239d0d97d3c84e0940",
    "1919b75119765621bb4f1e8",
    "09db12d731faee0b86df6b8",
    "488fc33df43fbdeea4eafb4",
    "827423ee40b675f756eb5fe",
    "abe197c484cb74757144a9a",
    "2b500e4bc0ec5a6d2bdbdd0",
    "c474aa53d70218761669360",
    "8eba1a13db3390bd6718cec",
    "753844673a27782cc42012e",
    "06ff83a145c37035a5c1268",
    "3b37417858cc2dd33ec3f62",
    "9a4a5a28ee17ca9c324842c",
    "bc29f465309c977e89610a4",
    "2663ae6ddf8b5ce2bb29488",
    "46f231efe457034c1814418",
    "3fb2ce85abe9b0c72e06fbe",
    "de87481f282c153971a0a2e",
    "fcd7ccf23c69fa99bba1412",
    "f0261447e9490ca8e474cec",
    "4410115818196f95cdd7012",
    "088fc31df4bfbde2a4eafb4",
    "b8fef1b6307729fb0a078c0",
    "5afea7acccb77bbc9d99a90",
    "49a7016ac653f65ecdc9076",
    "1944d085be4e7da8d6cc7d0",
    "251f62adc4032f0ee714002",
    "56471f8702a0721e00b12b8",
    "2b8e4923f2dd51e2d537fa0",
    "6b550a40a66f4755de95c26",
    "a18ad28d4e27fe92a4f6c84",
    "10c2e586388cb82a3d80758",
    "ef34a41817ee02133db2eb0",
    "7e9c0c54325a9c15836e000",
    "3693e572d1fde4cdf079e86",
    "bfb2cec5abe1b0c72e07fbe",
    "7ee18230c583cccc57d4b08",
    "a066cb2fedafc9f52664126",
    "bb23725abc47cc5f4cc4cd2",
    "ded9dba3bee40c59b5609b4",
    "d9a7016ac653e6decdc9036",
    "9ad46aed5f707f280ab5fc4",
    "e5921c77822587316d7d3c2",
    "4f14da8242a8b86dca73352",
    "8b8b507ad467d4441df770e",
    "22831c9cf1169467ad04b68",
    "213b838fe2ae54c38ee7180",
    "5d926b6dd71f085181a4e12",
    "66ab79d4b29ee6e69509e56",
    "958148682d748a38dd68baa",
    "b8ce020cf069c32a723ab14",
    "f4331d6d461607e95752746",
    "6da23ba424b9596133cf9c8",
    "a636bcbc7b30c5fbeae67fe",
    "5cb0d86a07df654a9089a20",
    "f11f106848780fc9ecdd80a",
    "1fbb5364fb8d2c9d730d5ba",
    "fcb86bc70a50c9d02a5d034",
    "a534433029eac15f322e34c",
    "c989d9c7c3d3b8c55d75130",
    "7bb38b2f0186d46643ae962",
    "2644ebadeb44b9467d1f42c",
    "608cc857594bfbb55d69600"
};

static const int Mn_ft8_174_91_[174][3] =
{
    {16,  45,  73},
    {25,  51,  62},
    {33,  58,  78},
    {1,  44,  45},
    {2,   7,  61},
    {3,   6,  54},
    {4,  35,  48},
    {5,  13,  21},
    {8,  56,  79},
    {9,  64,  69},
    {10,  19,  66},
    {11,  36,  60},
    {12,  37,  58},
    {14,  32,  43},
    {15,  63,  80},
    {17,  28,  77},
    {18,  74,  83},
    {22,  53,  81},
    {23,  30,  34},
    {24,  31,  40},
    {26,  41,  76},
    {27,  57,  70},
    {29,  49,  65},
    {3,  38,  78},
    {5,  39,  82},
    {46,  50,  73},
    {51,  52,  74},
    {55,  71,  72},
    {44,  67,  72},
    {43,  68,  78},
    {1,  32,  59},
    {2,   6,  71},
    {4,  16,  54},
    {7,  65,  67},
    {8,  30,  42},
    {9,  22,  31},
    {10,  18,  76},
    {11,  23,  82},
    {12,  28,  61},
    {13,  52,  79},
    {14,  50,  51},
    {15,  81,  83},
    {17,  29,  60},
    {19,  33,  64},
    {20,  26,  73},
    {21,  34,  40},
    {24,  27,  77},
    {25,  55,  58},
    {35,  53,  66},
    {36,  48,  68},
    {37,  46,  75},
    {38,  45,  47},
    {39,  57,  69},
    {41,  56,  62},
    {20,  49,  53},
    {46,  52,  63},
    {45,  70,  75},
    {27,  35,  80},
    {1,  15,  30},
    {2,  68,  80},
    {3,  36,  51},
    {4,  28,  51},
    {5,  31,  56},
    {6,  20,  37},
    {7,  40,  82},
    {8,  60,  69},
    {9,  10,  49},
    {11,  44,  57},
    {12,  39,  59},
    {13,  24,  55},
    {14,  21,  65},
    {16,  71,  78},
    {17,  30,  76},
    {18,  25,  80},
    {19,  61,  83},
    {22,  38,  77},
    {23,  41,  50},
    {7,  26,  58},
    {29,  32,  81},
    {33,  40,  73},
    {18,  34,  48},
    {13,  42,  64},
    {5,  26,  43},
    {47,  69,  72},
    {54,  55,  70},
    {45,  62,  68},
    {10,  63,  67},
    {14,  66,  72},
    {22,  60,  74},
    {35,  39,  79},
    {1,  46,  64},
    {1,  24,  66},
    {2,   5,  70},
    {3,  31,  65},
    {4,  49,  58},
    {1,   4,   5},
    {6,  60,  67},
    {7,  32,  75},
    {8,  48,  82},
    {9,  35,  41},
    {10,  39,  62},
    {11,  14,  61},
    {12,  71,  74},
    {13,  23,  78},
    {11,  35,  55},
    {15,  16,  79},
    {7,   9,  16},
    {17,  54,  63},
    {18,  50,  57},
    {19,  30,  47},
    {20,  64,  80},
    {21,  28,  69},
    {22,  25,  43},
    {13,  22,  37},
    {2,  47,  51},
    {23,  54,  74},
    {26,  34,  72},
    {27,  36,  37},
    {21,  36,  63},
    {29,  40,  44},
    {19,  26,  57},
    {3,  46,  82},
    {14,  15,  58},
    {33,  52,  53},
    {30,  43,  52},
    {6,   9,  52},
    {27,  33,  65},
    {25,  69,  73},
    {38,  55,  83},
    {20,  39,  77},
    {18,  29,  56},
    {32,  48,  71},
    {42,  51,  59},
    {28,  44,  79},
    {34,  60,  62},
    {31,  45,  61},
    {46,  68,  77},
    {6,  24,  76},
    {8,  10,  78},
    {40,  41,  70},
    {17,  50,  53},
    {42,  66,  68},
    {4,  22,  72},
    {36,  64,  81},
    {13,  29,  47},
    {2,   8,  81},
    {56,  67,  73},
    {5,  38,  50},
    {12,  38,  64},
    {59,  72,  80},
    {3,  26,  79},
    {45,  76,  81},
    {1,  65,  74},
    {7,  18,  77},
    {11,  56,  59},
    {14,  39,  54},
    {16,  37,  66},
    {10,  28,  55},
    {15,  60,  70},
    {17,  25,  82},
    {20,  30,  31},
    {12,  67,  68},
    {23,  75,  80},
    {27,  32,  62},
    {24,  69,  75},
    {19,  21,  71},
    {34,  53,  61},
    {35,  46,  47},
    {33,  59,  76},
    {40,  43,  83},
    {41,  42,  63},
    {49,  75,  83},
    {20,  44,  48},
    {42,  49,  57}
};

static const int Nm_ft8_174_91_[83][7] =
{
    {4,  31,  59,  91,  92,  96, 153},
    {5,  32,  60,  93, 115, 146,   0},
    {6,  24,  61,  94, 122, 151,   0},
    {7,  33,  62,  95,  96, 143,   0},
    {8,  25,  63,  83,  93,  96, 148},
    {6,  32,  64,  97, 126, 138,   0},
    {5,  34,  65,  78,  98, 107, 154},
    {9,  35,  66,  99, 139, 146,   0},
    {10,  36,  67, 100, 107, 126,   0},
    {11,  37,  67,  87, 101, 139, 158},
    {12,  38,  68, 102, 105, 155,   0},
    {13,  39,  69, 103, 149, 162,   0},
    {8,  40,  70,  82, 104, 114, 145},
    {14,  41,  71,  88, 102, 123, 156},
    {15,  42,  59, 106, 123, 159,   0},
    {1,  33,  72, 106, 107, 157,   0},
    {16,  43,  73, 108, 141, 160,   0},
    {17,  37,  74,  81, 109, 131, 154},
    {11,  44,  75, 110, 121, 166,   0},
    {45,  55,  64, 111, 130, 161, 173},
    {8,  46,  71, 112, 119, 166,   0},
    {18,  36,  76,  89, 113, 114, 143},
    {19,  38,  77, 104, 116, 163,   0},
    {20,  47,  70,  92, 138, 165,   0},
    {2,  48,  74, 113, 128, 160,   0},
    {21,  45,  78,  83, 117, 121, 151},
    {22,  47,  58, 118, 127, 164,   0},
    {16,  39,  62, 112, 134, 158,   0},
    {23,  43,  79, 120, 131, 145,   0},
    {19,  35,  59,  73, 110, 125, 161},
    {20,  36,  63,  94, 136, 161,   0},
    {14,  31,  79,  98, 132, 164,   0},
    {3,  44,  80, 124, 127, 169,   0},
    {19,  46,  81, 117, 135, 167,   0},
    {7,  49,  58,  90, 100, 105, 168},
    {12,  50,  61, 118, 119, 144,   0},
    {13,  51,  64, 114, 118, 157,   0},
    {24,  52,  76, 129, 148, 149,   0},
    {25,  53,  69,  90, 101, 130, 156},
    {20,  46,  65,  80, 120, 140, 170},
    {21,  54,  77, 100, 140, 171,   0},
    {35,  82, 133, 142, 171, 174,   0},
    {14,  30,  83, 113, 125, 170,   0},
    {4,  29,  68, 120, 134, 173,   0},
    {1,   4,  52,  57,  86, 136, 152},
    {26,  51,  56,  91, 122, 137, 168},
    {52,  84, 110, 115, 145, 168,   0},
    {7,  50,  81,  99, 132, 173,   0},
    {23,  55,  67,  95, 172, 174,   0},
    {26,  41,  77, 109, 141, 148,   0},
    {2,  27,  41,  61,  62, 115, 133},
    {27,  40,  56, 124, 125, 126,   0},
    {18,  49,  55, 124, 141, 167,   0},
    {6,  33,  85, 108, 116, 156,   0},
    {28,  48,  70,  85, 105, 129, 158},
    {9,  54,  63, 131, 147, 155,   0},
    {22,  53,  68, 109, 121, 174,   0},
    {3,  13,  48,  78,  95, 123,   0},
    {31,  69, 133, 150, 155, 169,   0},
    {12,  43,  66,  89,  97, 135, 159},
    {5,  39,  75, 102, 136, 167,   0},
    {2,  54,  86, 101, 135, 164,   0},
    {15,  56,  87, 108, 119, 171,   0},
    {10,  44,  82,  91, 111, 144, 149},
    {23,  34,  71,  94, 127, 153,   0},
    {11,  49,  88,  92, 142, 157,   0},
    {29,  34,  87,  97, 147, 162,   0},
    {30,  50,  60,  86, 137, 142, 162},
    {10,  53,  66,  84, 112, 128, 165},
    {22,  57,  85,  93, 140, 159,   0},
    {28,  32,  72, 103, 132, 166,   0},
    {28,  29,  84,  88, 117, 143, 150},
    {1,  26,  45,  80, 128, 147,   0},
    {17,  27,  89, 103, 116, 153,   0},
    {51,  57,  98, 163, 165, 172,   0},
    {21,  37,  73, 138, 152, 169,   0},
    {16,  47,  76, 130, 137, 154,   0},
    {3,  24,  30,  72, 104, 139,   0},
    {9,  40,  90, 106, 134, 151,   0},
    {15,  58,  60,  74, 111, 150, 163},
    {18,  42,  79, 144, 146, 152,   0},
    {25,  38,  65,  99, 122, 160,   0},
    {17,  42,  75, 129, 170, 172,   0}
};

static const int nrw_ft8_174_91[83] =
{
    7,6,6,6,7,6,7,6,6,7,6,6,7,7,6,6,
    6,7,6,7,6,7,6,6,6,7,6,6,6,7,6,6,
    6,6,7,6,6,6,7,7,6,6,6,6,7,7,6,6,
    6,6,7,6,6,6,7,6,6,6,6,7,6,6,6,7,
    6,6,6,7,7,6,6,7,6,6,6,6,6,6,6,7,
    6,6,6
};

/* ------------------------------------------------------------------ */
/* Global/static matrices (instance variables in MSHV)                */
/* ------------------------------------------------------------------ */

static bool first_osd174_91 = true;
// N=174, K=91
static char gen_osd174_91_[180][97];

static bool first_enc174_91_nocrc = true;
static char gen_osd174_91_nocrc[100][95];

/* Thread-local state for boxit91 / fetchit91 */
static thread_local int indexes_ft8_2_[2][5020];
static thread_local int fp_ft8_2[525020];
static thread_local int np_ft8_2[5020];
static thread_local int lastpat_ft8_2;
static thread_local int inext_ft8_2;

/* ------------------------------------------------------------------ */
/* Helper functions                                                    */
/* ------------------------------------------------------------------ */

static void platanh(double x, double &y)
{
    double isign = +1.0;
    double z = x;
    if (x < 0.0)
    {
        isign = -1.0;
        z = fabs(x);
    }
    if (z <= 0.664)
    {
        y = x / 0.83;
        return;
    }
    else if (z <= 0.9217)
    {
        y = isign * (z - 0.4064) / 0.322;
        return;
    }
    else if (z <= 0.9951)
    {
        y = isign * (z - 0.8378) / 0.0524;
        return;
    }
    else if (z <= 0.9998)
    {
        y = isign * (z - 0.9914) / 0.0012;
        return;
    }
    else
    {
        y = isign * 7.0;
        return;
    }
}

static void bshift1(bool *a, int cou_a, int ish)
{
    bool t[256];
    for (int i = 0; i < cou_a; i++)
        t[i] = a[i];
    if (ish > 0)
    {
        for (int i = 0; i < cou_a; i++)
        {
            if (i + ish < cou_a)
                a[i] = t[i + ish];
            else
                a[i] = t[i + ish - cou_a];
        }
    }
    if (ish < 0)
    {
        for (int i = 0; i < cou_a; i++)
        {
            if (i + ish < 0)
                a[i] = t[i + ish + cou_a];
            else
                a[i] = t[i + ish];
        }
    }
}

static void get_crc14(bool *mc, int len, int &ncrc)
{
    bool r[15 + 5];
    bool p[15] = {1,1,0,0,1,1,1,0,1,0,1,0,1,1,1};

    for (int i = 0; i < 15; ++i)
        r[i] = mc[i];

    for (int i = 0; i < len - 14; ++i)
    {
        r[14] = mc[i + 14];
        bool r0 = r[0];
        for (int j = 0; j < 15; ++j)
            r[j] = fmod(r[j] + r0 * p[j], 2);
        bshift1(r, 15, 1);
    }

    ncrc = 0;
    for (int i = 0; i < 14; ++i)
    {
        ncrc <<= 1;
        ncrc |= r[i];
    }
}

static void indexx_msk(double *arr, int n, int *indx)
{
    int M = 7;
    const int NSTACK = 50;
    int i, indxt, ir, itemp, j, jstack, k, l;
    int istack[NSTACK + 50];
    double a;

    for (j = 0; j <= n; j++)
        indx[j] = j;

    jstack = -1;
    l = 0;
    ir = n;
c1:
    if (ir - l < M)
    {
        for (j = l + 1; j <= ir; j++)
        {
            indxt = indx[j];
            a = arr[indxt];
            for (i = j - 1; i >= 0; i--)
            {
                if (arr[indx[i]] <= a) goto c2;
                indx[i + 1] = indx[i];
            }
            i = -1;
c2:
            indx[i + 1] = indxt;
        }
        if (jstack < 1) return;

        ir = istack[jstack];
        l = istack[jstack - 1];
        jstack = jstack - 2;
    }
    else
    {
        k = (l + ir) / 2;
        itemp = indx[k];
        indx[k] = indx[l + 1];
        indx[l + 1] = itemp;

        if (arr[indx[l + 1]] > arr[indx[ir]])
        {
            itemp = indx[l + 1];
            indx[l + 1] = indx[ir];
            indx[ir] = itemp;
        }
        if (arr[indx[l]] > arr[indx[ir]])
        {
            itemp = indx[l];
            indx[l] = indx[ir];
            indx[ir] = itemp;
        }
        if (arr[indx[l + 1]] > arr[indx[l]])
        {
            itemp = indx[l + 1];
            indx[l + 1] = indx[l];
            indx[l] = itemp;
        }

        i = l + 1;
        j = ir;
        indxt = indx[l];
        a = arr[indxt];
c3:
        i = i + 1;
        if (arr[indx[i]] < a) goto c3;

c4:
        j = j - 1;
        if (j < 0) j = 0;
        if (arr[indx[j]] > a) goto c4;
        if (j < i) goto c5;
        itemp = indx[i];
        indx[i] = indx[j];
        indx[j] = itemp;
        goto c3;

c5:
        indx[l] = indx[j];
        indx[j] = indxt;
        jstack = jstack + 2;
        if (jstack > NSTACK) return;
        if (ir - i + 1 >= j - l)
        {
            istack[jstack] = ir;
            istack[jstack - 1] = i;
            ir = j - 1;
        }
        else
        {
            istack[jstack] = j - 1;
            istack[jstack - 1] = l;
            l = i;
        }
    }
    goto c1;
}

static void mrbencode91(bool *me, bool *codeword, bool g2_[91][174], int N, int K)
{
    for (int i = 0; i < N; ++i) codeword[i] = 0;
    for (int i = 0; i < K; ++i)
    {
        if (me[i] == 1)
        {
            for (int j = 0; j < N; ++j)
                codeword[j] = (codeword[j] ^ g2_[i][j]);
        }
    }
}

static void nextpat_step1_91(bool *mi, int k, int iorder, int &iflag)
{
    if (iflag <= 0)
    {
        iflag = -1;
        return;
    }
    int beg = iflag - 1;
    int end = iflag + iorder;
    if (end > k) end = k;
    for (int i = beg; i < end; ++i)
    {
        if (i >= beg && i < beg + iorder)
            mi[i] = true;
        else
            mi[i] = false;
    }
    iflag--;
}

static bool any_ca_iand_ca_eq1_91(bool *a, bool *b, int count)
{
    bool res = false;
    for (int i = 0; i < count; ++i)
    {
        if ((a[i] & b[i]) == 1)
        {
            res = true;
            break;
        }
    }
    return res;
}

static void boxit91(bool &reset, bool *e2, int ntau, int npindex, int i1, int i2)
{
    if (reset)
    {
        for (int i = 0; i < 525000; ++i)
            fp_ft8_2[i] = -1;
        for (int i = 0; i < 5000; ++i)
        {
            np_ft8_2[i] = -1;
            indexes_ft8_2_[0][i] = -1;
            indexes_ft8_2_[1][i] = -1;
        }
        reset = false;
    }

    indexes_ft8_2_[0][npindex] = i1;
    indexes_ft8_2_[1][npindex] = i2;

    int ipat = 0;
    for (int i = 0; i < ntau; ++i)
    {
        if (e2[i] == 1)
            ipat += (1 << ((ntau - 1) - i));
    }

    int ip = fp_ft8_2[ipat];
    if (ip == -1)
        fp_ft8_2[ipat] = npindex;
    else
    {
        while (np_ft8_2[ip] != -1)
            ip = np_ft8_2[ip];
        np_ft8_2[ip] = npindex;
    }
}

static void fetchit91(bool &reset, bool *e2, int ntau, int &i1, int &i2)
{
    if (reset)
    {
        lastpat_ft8_2 = -1;
        reset = false;
    }
    int ipat = 0;
    for (int i = 0; i < ntau; ++i)
    {
        if (e2[i] == 1)
            ipat += (1 << ((ntau - 1) - i));
    }
    int index = fp_ft8_2[ipat];
    if (lastpat_ft8_2 != ipat && index >= 0)
    {
        i1 = indexes_ft8_2_[0][index];
        i2 = indexes_ft8_2_[1][index];
        inext_ft8_2 = np_ft8_2[index];
    }
    else if (lastpat_ft8_2 == ipat && inext_ft8_2 >= 0)
    {
        i1 = indexes_ft8_2_[0][inext_ft8_2];
        i2 = indexes_ft8_2_[1][inext_ft8_2];
        inext_ft8_2 = np_ft8_2[inext_ft8_2];
    }
    else
    {
        i1 = -1;
        i2 = -1;
        inext_ft8_2 = -1;
    }
    lastpat_ft8_2 = ipat;
}

static int hex_digit(char c)
{
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return 0;
}

static void encode174_91_nocrc(bool *message910, bool *codeword)
{
    const int N = 174;
    const int K = 91;
    const int M = N - K;
    bool pchecks[95];

    if (first_enc174_91_nocrc)
    {
        for (int i = 0; i < 95; ++i)
        {
            for (int j = 0; j < 85; ++j)
                gen_osd174_91_nocrc[i][j] = 0;
        }
        for (int i = 0; i < M; ++i)
        {
            for (int j = 0; j < 23; ++j)
            {
                int istr = hex_digit(g_ft8_174_91[i][j]);
                for (int jj = 0; jj < 4; ++jj)
                {
                    int irow = j * 4 + jj;
                    if (irow <= 90)
                        gen_osd174_91_nocrc[irow][i] = (1 & (istr >> (3 - jj)));
                }
            }
        }
        first_enc174_91_nocrc = false;
    }

    for (int i = 0; i < 83; ++i)
    {
        int nsum = 0;
        for (int j = 0; j < 91; ++j)
            nsum += message910[j] * gen_osd174_91_nocrc[j][i];
        pchecks[i] = fmod(nsum, 2);
    }

    for (int i = 0; i < K; ++i) codeword[i] = message910[i];
    for (int i = 0; i < M; ++i) codeword[i + 91] = pchecks[i];
}

static void osd174_91_1(double *llr, bool *apmask, int ndeep,
                        bool *message91, bool *cw, int &nhardmin, double &dmin)
{
    const int N = 174;
    const int K = 91;
    const int M = N - K;
    double rx[N];
    bool apmaskr[N];
    bool apmaskr2[N];
    bool hdec[N + 2];
    bool hdec2[N + 2];
    double absrx[N];
    double absrx2[N];
    bool genmrb_[N][K];
    bool g2_[K][N];
    int indices[N];
    int indx[N];
    bool temp[K];
    bool m0[K];
    bool c0[N];
    bool nxor[N + 2];
    bool misub[K + 8];
    bool me[K], mi[K];
    bool ce[N];
    bool e2sub[M];
    bool e2[M];
    bool ui[M];
    bool r2pat[M];
    bool cw_t[N];
    bool m96[96 + 5];

    if (first_osd174_91)
    {
        for (int i = 0; i < N; ++i)
            for (int j = 0; j < K; ++j)
                gen_osd174_91_[i][j] = 0;

        for (int i = 0; i < K; ++i)
        {
            bool message910[120];
            for (int j = 0; j < 91; ++j)
                message910[j] = 0;
            message910[i] = 1;
            encode174_91_nocrc(message910, cw);
            for (int j = 0; j < N; ++j)
                gen_osd174_91_[j][i] = cw[j];
        }
        first_osd174_91 = false;
    }

    for (int i = 0; i < N; ++i)
    {
        rx[i] = llr[i];
        apmaskr[i] = apmask[i];
        hdec[i] = 0;
        if (rx[i] >= 0.0) hdec[i] = 1;
        absrx[i] = fabs(rx[i]);
    }

    indexx_msk(absrx, N - 1, indx);

    for (int i = 0; i < N; ++i)
    {
        for (int j = 0; j < K; ++j)
            genmrb_[i][j] = gen_osd174_91_[indx[(N - 1) - i]][j];
        indices[i] = indx[(N - 1) - i];
    }

    int iflag = 0;
    for (int id = 0; id < K; ++id)
    {
        for (int icol = id; icol < K + 20; ++icol)
        {
            iflag = 0;
            if (genmrb_[icol][id] == 1)
            {
                iflag = 1;
                if (icol != id)
                {
                    for (int z = 0; z < K; ++z)
                    {
                        temp[z] = genmrb_[id][z];
                        genmrb_[id][z] = genmrb_[icol][z];
                        genmrb_[icol][z] = temp[z];
                    }
                    int itmp = indices[id];
                    indices[id] = indices[icol];
                    indices[icol] = itmp;
                }
                for (int ii = 0; ii < K; ++ii)
                {
                    if (ii != id && genmrb_[id][ii] == 1)
                    {
                        for (int z = 0; z < N; ++z)
                            genmrb_[z][ii] = (genmrb_[z][ii] ^ genmrb_[z][id]);
                    }
                }
                break;
            }
        }
    }

    for (int i = 0; i < N; ++i)
        for (int j = 0; j < K; ++j)
            g2_[j][i] = genmrb_[i][j];

    for (int i = 0; i < N; ++i)
    {
        hdec2[i] = hdec[indices[i]];
        absrx2[i] = absrx[indices[i]];
        apmaskr2[i] = apmaskr[indices[i]];
    }
    for (int i = 0; i < K; ++i)
        m0[i] = hdec2[i];

    mrbencode91(m0, c0, g2_, N, K);

    nhardmin = 0;
    dmin = 0.0;
    for (int i = 0; i < N; ++i)
    {
        nxor[i] = (c0[i] ^ hdec2[i]);
        nhardmin += nxor[i];
        dmin += (double)nxor[i] * absrx2[i];
    }

    for (int i = 0; i < N; ++i)
        cw[i] = c0[i];

    int nt = 0;
    int nrejected = 0;
    int nord = 0;
    int npre1 = 0;
    int npre2 = 0;
    int ntheta = 0;
    int ntau = 0;
    int ntotal = 0;

    if (ndeep == 0) goto c998;
    if (ndeep > 6) ndeep = 6;
    if (ndeep == 1)
    {
        nord = 1; npre1 = 0; npre2 = 0; nt = 40; ntheta = 12;
    }
    else if (ndeep == 2)
    {
        nord = 1; npre1 = 1; npre2 = 0; nt = 40; ntheta = 10;
    }
    else if (ndeep == 3)
    {
        nord = 1; npre1 = 1; npre2 = 1; nt = 40; ntheta = 12; ntau = 14;
    }
    else if (ndeep == 4)
    {
        nord = 2; npre1 = 1; nt = 40; ntheta = 12; npre2 = 1; ntau = 17;
    }
    else if (ndeep == 5)
    {
        npre1 = 1; npre2 = 1; nt = 40; ntheta = 12;
        nord = 3; ntau = 15;
    }
    else
    {
        nord = 4; npre1 = 1; npre2 = 1; nt = 95; ntheta = 12; ntau = 15;
    }

    for (int iorder = 1; iorder <= nord; ++iorder)
    {
        for (int z = 0; z < K - iorder; ++z)
            misub[z] = 0;
        for (int z = K - iorder; z < K; ++z)
            misub[z] = 1;
        iflag = K - iorder;

        while (iflag >= 0)
        {
            int iend = 0;
            if (iorder == nord && npre1 == 0)
                iend = iflag;
            else
                iend = 0;

            double d1 = 0.0;
            for (int n1 = iflag; n1 >= iend; --n1)
            {
                for (int x = 0; x < K; ++x)
                    mi[x] = misub[x];
                mi[n1] = 1;
                if (any_ca_iand_ca_eq1_91(apmaskr2, mi, K)) continue;
                ntotal++;
                for (int x = 0; x < K; ++x)
                    me[x] = (m0[x] ^ mi[x]);
                int nd1Kpt = 0;
                if (n1 == iflag)
                {
                    mrbencode91(me, ce, g2_, N, K);
                    for (int x = 0; x < M; ++x)
                    {
                        e2sub[x] = (ce[K + x] ^ hdec2[K + x]);
                        e2[x] = e2sub[x];
                    }
                    nd1Kpt = 0;
                    for (int x = 0; x < nt; ++x)
                        nd1Kpt += e2sub[x];
                    nd1Kpt = nd1Kpt + 1;
                    d1 = 0.0;
                    for (int x = 0; x < K; ++x)
                        d1 += (double)(me[x] ^ hdec2[x]) * absrx2[x];
                }
                else
                {
                    for (int x = 0; x < M; ++x)
                        e2[x] = (e2sub[x] ^ g2_[n1][K + x]);
                    nd1Kpt = 0;
                    for (int x = 0; x < nt; ++x)
                        nd1Kpt += e2[x];
                    nd1Kpt = nd1Kpt + 2;
                }
                if (nd1Kpt <= ntheta)
                {
                    mrbencode91(me, ce, g2_, N, K);
                    for (int x = 0; x < N; ++x)
                        nxor[x] = (ce[x] ^ hdec2[x]);
                    double dd = 0.0;
                    if (n1 == iflag)
                    {
                        dd = 0.0;
                        for (int x = 0; x < M; ++x)
                            dd += (double)e2sub[x] * absrx2[K + x];
                        dd = d1 + dd;
                    }
                    else
                    {
                        dd = 0.0;
                        for (int x = 0; x < M; ++x)
                            dd += (double)e2[x] * absrx2[K + x];
                        dd = d1 + (double)(ce[n1] ^ hdec2[n1]) * absrx2[n1] + dd;
                    }
                    if (dd < dmin)
                    {
                        dmin = dd;
                        for (int x = 0; x < N; ++x)
                            cw[x] = ce[x];
                        nhardmin = 0;
                        for (int x = 0; x < N; ++x)
                            nhardmin += nxor[x];
                    }
                }
                else
                    nrejected++;
            }
            nextpat_step1_91(misub, K, iorder, iflag);
        }
    }

    if (npre2 == 1)
    {
        bool reset = true;
        ntotal = 0;
        for (int i1 = K - 1; i1 >= 0; --i1)
        {
            for (int i2 = i1 - 1; i2 >= 0; --i2)
            {
                for (int x = 0; x < ntau; ++x)
                    mi[x] = (g2_[i1][K + x] ^ g2_[i2][K + x]);
                boxit91(reset, mi, ntau, ntotal, i1, i2);
                ntotal++;
            }
        }

        reset = true;
        for (int z = 0; z < K - nord; ++z)
            misub[z] = 0;
        for (int z = K - nord; z < K; ++z)
            misub[z] = 1;
        iflag = K - nord;

        while (iflag >= 0)
        {
            for (int z = 0; z < K; ++z)
                me[z] = (m0[z] ^ misub[z]);
            mrbencode91(me, ce, g2_, N, K);
            for (int z = 0; z < M; ++z)
                e2sub[z] = (ce[K + z] ^ hdec2[K + z]);
            for (int i2 = -1; i2 < ntau; ++i2)
            {
                for (int x = 0; x < M; ++x)
                    ui[x] = 0;
                if (i2 > -1) ui[i2] = 1;
                for (int x = 0; x < M; ++x)
                    r2pat[x] = (e2sub[x] ^ ui[x]);

c778:
                int in1 = -1;
                int in2 = -1;
                fetchit91(reset, r2pat, ntau, in1, in2);
                if (in1 >= 0 && in2 >= 0)
                {
                    for (int z = 0; z < K; ++z)
                        mi[z] = misub[z];
                    mi[in1] = 1;
                    mi[in2] = 1;
                    int sum_mi = 0;
                    for (int z = 0; z < K; ++z)
                        sum_mi += mi[z];
                    if (sum_mi < nord + npre1 + npre2 || any_ca_iand_ca_eq1_91(apmaskr2, mi, K))
                        continue;
                    for (int z = 0; z < K; ++z)
                        me[z] = (m0[z] ^ mi[z]);
                    mrbencode91(me, ce, g2_, N, K);
                    for (int z = 0; z < N; ++z)
                        nxor[z] = (ce[z] ^ hdec2[z]);
                    double dd = 0.0;
                    for (int z = 0; z < N; ++z)
                        dd += (double)nxor[z] * absrx2[z];
                    if (dd < dmin)
                    {
                        dmin = dd;
                        for (int z = 0; z < N; ++z)
                            cw[z] = ce[z];
                        nhardmin = 0;
                        for (int x = 0; x < N; ++x)
                            nhardmin += nxor[x];
                    }
                    goto c778;
                }
            }
            nextpat_step1_91(misub, K, nord, iflag);
        }
    }

c998:
    for (int i = 0; i < N; ++i)
        cw_t[indices[i]] = cw[i];
    for (int i = 0; i < N; ++i)
    {
        cw[i] = cw_t[i];
        if (i < 91)
            message91[i] = cw_t[i];
        if (i < 96)
        {
            m96[i] = 0;
            if (i < 77) m96[i] = cw[i];
            if (i > 81) m96[i] = cw[i - 5];
        }
    }

    int nbadcrc;
    get_crc14(m96, 96, nbadcrc);
    if (nbadcrc != 0) nhardmin = -nhardmin;
}

static void decode174_91(double *llr, int maxosd, int norder, bool *apmask,
                         bool *message91, bool *cw, int &nharderror, double &dmin)
{
    const int N = 174;
    const int K = 91;
    const int M = (N - K);
    double zsave_[4][N];
    double tov_[N][3];
    double toc_[M][7];
    double tanhtoc_[M][7];
    double zsum[N + 5];
    double zn[N + 5];
    const int ncw = 3;
    bool m96[96 + 5];
    bool hdec[N + 2];
    bool nxor[N + 2];
    int synd[M + 5];

    int nosd = 0;
    if (maxosd > 3) maxosd = 3;
    if (maxosd == 0)
    {
        nosd = 1;
        for (int i = 0; i < N; ++i)
            zsave_[0][i] = llr[i];
    }
    else if (maxosd > 0)
        nosd = maxosd;
    else if (maxosd < 0)
        nosd = 0;

    for (int i = 0; i < N; ++i)
    {
        zsum[i] = 0.0;
        for (int j = 0; j < 3; ++j)
            tov_[i][j] = 0.0;
    }

    for (int j = 0; j < M; ++j)
    {
        for (int i = 0; i < (nrw_ft8_174_91[j]); ++i)
            toc_[j][i] = llr[Nm_ft8_174_91_[j][i] - 1];
    }

    int ncnt = 0;
    int nclast = 0;
    for (int iter = 0; iter <= 30; ++iter)
    {
        for (int i = 0; i < N; ++i)
        {
            if (apmask[i] != 1)
            {
                double sumt = 0.0;
                for (int x = 0; x < ncw; ++x)
                    sumt += tov_[i][x];
                zn[i] = llr[i] + sumt;
            }
            else
                zn[i] = llr[i];

            if (zn[i] > 0.0)
                cw[i] = 1;
            else
                cw[i] = 0;

            zsum[i] += zn[i];
        }

        if (iter > 0 && iter <= maxosd)
        {
            for (int i = 0; i < N; ++i)
                zsave_[iter - 1][i] = zsum[i];
        }

        int ncheck = 0;
        for (int i = 0; i < M; ++i)
        {
            int sum = 0;
            for (int x = 0; x < nrw_ft8_174_91[i]; ++x)
                sum += (int)cw[Nm_ft8_174_91_[i][x] - 1];
            synd[i] = sum;
            if (fmod(synd[i], 2) != 0)
                ncheck++;
        }

        if (ncheck == 0)
        {
            int nbadcrc;
            for (int i = 0; i < 96; ++i)
            {
                m96[i] = 0;
                if (i < 77) m96[i] = cw[i];
                if (i > 81) m96[i] = cw[i - 5];
            }
            get_crc14(m96, 96, nbadcrc);
            if (nbadcrc == 0)
            {
                int count = 0;
                for (int i = 0; i < N; ++i)
                {
                    if ((double)(2 * cw[i] - 1) * llr[i] < 0.0)
                        count++;
                }
                nharderror = count;
                for (int i = 0; i < 91; ++i)
                    message91[i] = cw[i];
                for (int i = 0; i < N; ++i)
                {
                    hdec[i] = 0;
                    if (llr[i] >= 0.0) hdec[i] = 1;
                    nxor[i] = hdec[i] ^ cw[i];
                    dmin += (double)nxor[i] * fabs(llr[i]);
                }
                return;
            }
        }

        if (iter > 0)
        {
            int nd = ncheck - nclast;
            if (nd < 0)
                ncnt = 0;
            else
                ncnt++;
            if (ncnt >= 5 && iter >= 10 && ncheck > 15)
            {
                nharderror = -1;
                break;
            }
        }
        nclast = ncheck;

        for (int j = 0; j < M; ++j)
        {
            for (int i = 0; i < nrw_ft8_174_91[j]; ++i)
            {
                int ibj = Nm_ft8_174_91_[j][i] - 1;
                toc_[j][i] = zn[ibj];
                for (int kk = 0; kk < ncw; kk++)
                {
                    if (Mn_ft8_174_91_[ibj][kk] - 1 == j)
                        toc_[j][i] = toc_[j][i] - tov_[ibj][kk];
                }
            }
        }

        for (int i = 0; i < M; ++i)
            for (int x = 0; x < 7; ++x)
                tanhtoc_[i][x] = tanh(-toc_[i][x] / 2.0);

        for (int j = 0; j < N; ++j)
        {
            for (int i = 0; i < ncw; ++i)
            {
                int ichk = Mn_ft8_174_91_[j][i] - 1;
                double Tmn = 1.0;
                for (int z = 0; z < nrw_ft8_174_91[ichk]; ++z)
                {
                    if (Nm_ft8_174_91_[ichk][z] - 1 != j)
                        Tmn = Tmn * tanhtoc_[ichk][z];
                }
                double y;
                platanh(-Tmn, y);
                tov_[j][i] = 2.0 * y;
            }
        }
    }

    for (int io = 0; io < nosd; ++io)
    {
        for (int j = 0; j < N; ++j)
            zn[j] = zsave_[io][j];
        double dminosd = 0.0;
        osd174_91_1(zn, apmask, norder, message91, cw, nharderror, dminosd);
        if (nharderror > 0)
        {
            for (int i = 0; i < N; ++i)
            {
                hdec[i] = 0;
                if (llr[i] >= 0.0) hdec[i] = 1;
                nxor[i] = hdec[i] ^ cw[i];
                dmin += (double)nxor[i] * fabs(llr[i]);
            }
            return;
        }
    }
    nharderror = -1;
}

/* ------------------------------------------------------------------ */
/* C-compatible wrapper                                                */
/* ------------------------------------------------------------------ */

extern "C" void mshv_decode174_91(double *llr, int maxosd, int norder, int *apmask,
                                  int *message91, int *cw, int *nharderror, double *dmin)
{
    const int N = 174;
    const int K = 91;
    bool apmask_b[N];
    bool message91_b[K];
    bool cw_b[N];
    int nhard;
    double dmin_out = 0.0;

    for (int i = 0; i < N; ++i)
        apmask_b[i] = (apmask[i] != 0);

    decode174_91(llr, maxosd, norder, apmask_b, message91_b, cw_b, nhard, dmin_out);

    for (int i = 0; i < K; ++i)
        message91[i] = message91_b[i] ? 1 : 0;
    for (int i = 0; i < N; ++i)
        cw[i] = cw_b[i] ? 1 : 0;
    *nharderror = nhard;
    *dmin = dmin_out;
}
