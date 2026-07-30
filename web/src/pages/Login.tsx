import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  Divider,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { useEffect, useState, useCallback } from "react";
import { useForm } from "react-hook-form";

import { getAuthProviders } from "../api";
import { useToast } from "../hooks/useToast";

interface LoginProps {
  onLogin: () => void;
}

interface LoginFormData {
  username: string;
  password: string;
}

export default function Login({ onLogin }: LoginProps) {
  const [oauthEnabled, setOauthEnabled] = useState(false);
  const { showToast } = useToast();

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormData>();

  useEffect(() => {
    const loadProviders = async () => {
      try {
        const providers = await getAuthProviders();
        setOauthEnabled(providers.oauth);
      } catch (error) {
        console.error("Failed to fetch auth providers", error);
      }
    };

    void loadProviders();
  }, []);

  const handleLocalLogin = useCallback(
    async (data: LoginFormData) => {
      try {
        const response = await fetch("/api/auth/login?method=local", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include",
          body: JSON.stringify({
            username: data.username.trim(),
            password: data.password,
          }),
        });

        if (!response.ok) {
          if (response.status === 401) {
            setError("password", {
              type: "server",
              message: "Invalid username or password",
            });
            return;
          }

          throw new Error(`Login failed (${response.status.toString()})`);
        }

        onLogin();
      } catch (error) {
        console.error("Local login failed", error);
        showToast({
          message: "Login failed. Please try again.",
          severity: "error",
        });
      }
    },
    [onLogin, setError, showToast],
  );

  return (
    <Box
      sx={{
        minHeight: "100dvh",
        display: "flex",
        alignItems: "center",
      }}
    >
      <Container maxWidth="sm">
        <Card elevation={2}>
          <CardContent>
            <Stack spacing={3}>
              <Stack
                direction="row"
                spacing={1.25}
                sx={{ alignItems: "center", justifyContent: "center" }}
              >
                <Typography variant="h4" component="h1" noWrap sx={{ fontWeight: 700 }}>
                  ADOverseas
                </Typography>
              </Stack>

              <Typography color="text.secondary" sx={{ textAlign: "center" }}>
                View and manage current booked overseas travels
              </Typography>

              <Stack spacing={2}>
                <Button
                  component="a"
                  href="/api/auth/login"
                  variant="contained"
                  fullWidth
                  disabled={!oauthEnabled}
                >
                  Sign in with OAuth
                </Button>

                {!oauthEnabled && (
                  <Typography variant="caption" color="text.secondary" sx={{ textAlign: "center" }}>
                    OAuth sign-in is disabled. Use your local administrator credentials instead.
                  </Typography>
                )}
              </Stack>

              <Divider />

              <Stack
                component="form"
                spacing={2}
                onSubmit={(e) => void handleSubmit(handleLocalLogin)(e)}
              >
                <TextField
                  label="Username"
                  fullWidth
                  required
                  {...register("username")}
                  error={Boolean(errors.username)}
                  helperText={errors.username?.message}
                />

                <TextField
                  label="Password"
                  type="password"
                  fullWidth
                  required
                  {...register("password")}
                  error={Boolean(errors.password)}
                  helperText={errors.password?.message}
                />

                <Button type="submit" variant="contained" fullWidth disabled={isSubmitting}>
                  {isSubmitting ? "Signing In..." : "Sign In"}
                </Button>
              </Stack>
            </Stack>
          </CardContent>
        </Card>
      </Container>
    </Box>
  );
}
