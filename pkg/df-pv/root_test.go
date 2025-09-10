package df_pv

import (
    "testing"
    "k8s.io/apimachinery/pkg/api/resource"
)

func TestConvertQuantityValueToHumanReadableIECString(t *testing.T) {
    tests := []struct {
        name  string
        bytes int64
        want  string
    }{
        {name: "zero", bytes: 0, want: "0"},
        {name: "lt_1Ki", bytes: 512, want: "512"},
        {name: "eq_1Ki", bytes: 1 << 10, want: "1Ki"},
        {name: "one_point_five_Ki", bytes: 1536, want: "1.5Ki"},
        {name: "eq_1Mi", bytes: 1 << 20, want: "1Mi"},
        {name: "eq_1Gi", bytes: 1 << 30, want: "1Gi"},
        {name: "one_point_five_Gi", bytes: (1 << 30) + (512 << 20), want: "1.5Gi"},
        {name: "eq_1Ti", bytes: 1 << 40, want: "1Ti"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            q := resource.NewQuantity(tt.bytes, resource.BinarySI)
            got := ConvertQuantityValueToHumanReadableIECString(q)
            if got != tt.want {
                t.Fatalf("ConvertQuantityValueToHumanReadableIECString(%d) = %q, want %q", tt.bytes, got, tt.want)
            }
        })
    }
}

