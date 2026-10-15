export type AuthUser = {
  id: number;
  name: string;
  email: string;
  created_at: string;
  email_verified: boolean;
  has_password: boolean;
  google_linked: boolean;
};

export type LoginPayload = {
  email: string;
  password: string;
};

export type RegisterPayload = {
  name: string;
  email: string;
  password: string;
};
