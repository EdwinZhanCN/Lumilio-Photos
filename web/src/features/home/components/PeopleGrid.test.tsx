import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PeopleGrid from "./PeopleGrid";

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      switch (key) {
        case "home.people.eyebrow":
          return "People";
        case "home.people.title":
          return "Recognized Faces";
        case "home.people.subtitle":
          return "Subtitle";
        case "home.people.empty":
          return "No people";
        case "people.unnamed":
          return "Unnamed";
        case "people.confirmed":
          return "Named";
        case "people.photosCount":
          return `${options?.count ?? 0} photos`;
        default:
          return key;
      }
    },
  }),
}));

vi.mock("@/lib/assets/assetUrls", () => ({
  assetUrls: {
    getPersonCoverUrl: (id: number) => `/api/v1/people/${id}/cover`,
  },
}));

describe("PeopleGrid", () => {
  it("renders named and unnamed people and handles clicks", () => {
    const onPersonClick = vi.fn();

    render(
      <PeopleGrid
        people={[
          {
            person_id: 1,
            name: "Alice",
            is_confirmed: true,
            asset_count: 4,
            cover_face_image_path: ".lumilio/assets/faces/1.webp",
          },
          {
            person_id: 2,
            name: undefined,
            is_confirmed: false,
            asset_count: 1,
            cover_face_image_path: undefined,
          },
        ]}
        onPersonClick={onPersonClick}
      />,
    );

    expect(screen.getByText("Recognized Faces")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Unnamed")).toBeInTheDocument();
    expect(screen.getByText("4 photos")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Alice/i }));
    expect(onPersonClick).toHaveBeenCalledTimes(1);
  });
});
