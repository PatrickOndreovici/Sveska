import type { RouteObject } from "react-router";
import { LoginPage } from "../features/auth/LoginPage";
import { GuestOnly, RequireAuth } from "../features/auth/routeGuards";
import { SignUpPage } from "../features/auth/SignUpPage";
import { VocabularyPage } from "../features/vocabulary/VocabularyPage";
import { HomePage } from "../pages/HomePage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { RootLayout } from "./RootLayout";

export const routes: RouteObject[] = [
  {
    element: <RootLayout />,
    children: [
      {
        element: <RequireAuth />,
        children: [
          { index: true, element: <HomePage /> },
          { path: "vocabulary", element: <VocabularyPage /> },
        ],
      },
      {
        element: <GuestOnly />,
        children: [
          { path: "login", element: <LoginPage /> },
          { path: "signup", element: <SignUpPage /> },
        ],
      },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
];
