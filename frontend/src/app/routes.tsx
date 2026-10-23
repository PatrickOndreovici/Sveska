import type { RouteObject } from "react-router";
import { LoginPage } from "../features/auth/LoginPage";
import { DocumentPage } from "../features/lessons/DocumentPage";
import { LessonPage } from "../features/lessons/LessonPage";
import { LessonsPage } from "../features/lessons/LessonsPage";
import { GuestOnly, RequireAuth, RequireRole } from "../features/auth/routeGuards";
import { SignUpPage } from "../features/auth/SignUpPage";
import { SettingsPage } from "../features/settings/SettingsPage";
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
          { path: "settings", element: <SettingsPage /> },
          {
            // A student's own study material; teachers will use classes and assignments.
            element: <RequireRole role="student" />,
            children: [
              { path: "lessons", element: <LessonsPage /> },
              { path: "lessons/:lessonId", element: <LessonPage /> },
              { path: "documents/:documentId", element: <DocumentPage /> },
              { path: "vocabulary", element: <VocabularyPage /> },
            ],
          },
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
