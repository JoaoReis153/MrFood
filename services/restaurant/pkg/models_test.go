package pkg

import "testing"

func TestRestaurantValidateCreateRequest(t *testing.T) {
	tests := []struct {
		name            string
		r               *Restaurant
		wantErr         bool
		wantOpeningTime string
		wantClosingTime string
	}{
		{
			name: "valid payload",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   20,
				OpeningTime: "09:00:00",
				ClosingTime: "17:00:00",
				Categories:  []string{"Sushi", "Ramen"},
			},
			wantErr:         false,
			wantOpeningTime: "09:00:00",
			wantClosingTime: "17:00:00",
		},
		{
			name: "rounds up minutes and seconds",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   20,
				OpeningTime: "09:10:01",
				ClosingTime: "17:01:59",
				Categories:  []string{"Sushi", "Ramen"},
			},
			wantErr:         false,
			wantOpeningTime: "10:00:00",
			wantClosingTime: "18:00:00",
		},
		{
			name: "reject invalid latitude",
			r: &Restaurant{
				Latitude:    91,
				Longitude:   20,
				OpeningTime: "09:00:00",
				ClosingTime: "17:00:00",
			},
			wantErr: true,
		},
		{
			name: "reject invalid longitude",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   181,
				OpeningTime: "09:00:00",
				ClosingTime: "17:00:00",
			},
			wantErr: true,
		},
		{
			name: "reject missing opening time",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   20,
				ClosingTime: "17:00:00",
			},
			wantErr: true,
		},
		{
			name: "reject invalid opening time format",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   20,
				OpeningTime: "9:00",
				ClosingTime: "17:00:00",
			},
			wantErr: true,
		},
		{
			name: "reject duplicate categories",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   20,
				OpeningTime: "09:00:00",
				ClosingTime: "17:00:00",
				Categories:  []string{"Sushi", " sushi "},
			},
			wantErr: true,
		},
		{
			name: "reject when opening is not earlier than closing",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   20,
				OpeningTime: "17:00:00",
				ClosingTime: "17:00:00",
			},
			wantErr: true,
		},
		{
			name: "reject when rounding makes times equal",
			r: &Restaurant{
				Latitude:    10,
				Longitude:   20,
				OpeningTime: "09:30:00",
				ClosingTime: "09:40:00",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.ValidateCreateRequest()
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr {
				if tt.r.OpeningTime != tt.wantOpeningTime {
					t.Fatalf("expected opening_time=%s, got %s", tt.wantOpeningTime, tt.r.OpeningTime)
				}
				if tt.r.ClosingTime != tt.wantClosingTime {
					t.Fatalf("expected closing_time=%s, got %s", tt.wantClosingTime, tt.r.ClosingTime)
				}
			}
		})
	}
}
