## Page Design Protocol

### Header
- The header should be consistent across all pages.
- It should include the icon on the left and navigation title on the right.

1. Using Heroicons for the icon is recommended.
standard className should be `w-6 h-6 text-primary`.
```jsx
<PageHeader
  title="Studio"
  icon={<PaintBrushIcon className="w-6 h-6 text-primary" />}
></PageHeader>
```
