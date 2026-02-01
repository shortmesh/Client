package main

// func TestFormatUsername(t *testing.T) {
// 	// Create a test configuration
// 	conf := &Conf{
// 		HomeServerDomain: "example.com",
// 		Bridges: []map[string]BridgeConfig{
// 			{
// 				"telegram": {
// 					UsernameTemplate: "@{{.}}",
// 				},
// 				"discord": {
// 					UsernameTemplate: "{{.}}",
// 				},
// 			},
// 		},
// 	}

// 	tests := []struct {
// 		name        string
// 		bridgeType  string
// 		username    string
// 		want        string
// 		expectError bool
// 	}{
// 		{
// 			name:        "Valid Telegram username",
// 			bridgeType:  "telegram",
// 			username:    "testuser",
// 			want:        "@testuser:example.com",
// 			expectError: false,
// 		},
// 		{
// 			name:        "Valid Discord username",
// 			bridgeType:  "discord",
// 			username:    "testuser",
// 			want:        "@testuser:example.com",
// 			expectError: false,
// 		},
// 		{
// 			name:        "Invalid bridge type",
// 			bridgeType:  "nonexistent",
// 			username:    "testuser",
// 			want:        "",
// 			expectError: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := conf.FormatUsername(tt.bridgeType, tt.username)
// 			if (err != nil) != tt.expectError {
// 				t.Errorf("FormatUsername() error = %v, expectError %v", err, tt.expectError)
// 				return
// 			}
// 			if got != tt.want {
// 				t.Errorf("FormatUsername() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
