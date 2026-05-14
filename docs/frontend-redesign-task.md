# Frontend Redesign Task

## Goal

Redesign the fund tracking frontend into a unified, clean, light dashboard-style product interface.

Use `DESIGN.md` as the visual design reference, but do not copy it mechanically. Adapt its principles to this project.

The current problem is not business logic. The current problem is inconsistent UI style, strange login/register layout, mixed Chinese/English labels, and inconsistent navigation/page structure.

## Hard constraints

Do not change backend code.

Do not change existing API contracts.

Do not change existing authentication logic.

Do not change existing fund/search/watchlist/profile/update business logic.

Do not rename existing API functions unless absolutely necessary.

Do not introduce a large UI framework.

Keep the existing Next.js Pages Router structure.

Prefer existing CSS Modules.

Do not convert the project to App Router.

Do not rewrite the entire frontend from scratch.

Do not remove existing working features.

## Pages to optimize

Optimize these pages as one unified product:

- `/login`
- `/register`
- `/about`
- `/profile`
- shared dashboard shell
- shared navigation/menu/header/sidebar if present

## Language requirement

All user-facing UI text should be Chinese.

Examples:

- Login -> 登录
- Register -> 注册
- Profile -> 账户
- Watchlist -> 关注列表
- Funds -> 基金
- Search -> 搜索
- Logout -> 退出登录
- Update -> 更新数据

Do not leave mixed English/Chinese labels in the visible UI unless the English term is a product name or technical identifier.

## Visual direction

Use a clean, practical SaaS/dashboard style.

The interface should feel like a lightweight financial dashboard, not a game page, landing page, or particle-effect demo.

Prefer:

- light background
- clear cards
- readable spacing
- restrained borders
- consistent typography
- consistent button styles
- consistent form styles
- simple navigation
- obvious active states
- clear hierarchy between title, subtitle, content, and actions

Avoid:

- dark particle backgrounds
- decorative texture backgrounds
- inconsistent pill navigation
- oversized empty areas
- strange centered login layout
- excessive gradients
- unrelated animations
- mixing multiple visual styles

## Login/Register requirements

The login and register pages should look like part of the same product.

They should use:

- centered but not awkward layout
- clean card container
- clear title and subtitle
- consistent input styles
- consistent primary button
- clear secondary link between login and register
- Chinese labels and placeholders

The login page should not look detached from the rest of the dashboard.

## Dashboard/navigation requirements

Unify navigation labels into Chinese.

Navigation should be simple and predictable.

The same layout shell should be reused where appropriate.

Active page state should be visible.

The page header should clearly show the current page purpose.

Do not keep multiple competing navigation patterns.

## Funds/About page requirements

If `/about` is currently being used as the funds page, keep the route unless changing the route is already supported safely.

The page should clearly present fund search and fund list/watch actions.

Do not break:

- getFunds
- getFund
- searchFunds
- updateFunds
- add/remove watchlist behavior

## Profile/Watchlist page requirements

The profile page should clearly separate:

- account information
- watchlist
- threshold settings
- logout action

Do not change the underlying data logic.

Improve only layout, labels, spacing, and visual consistency.

## Code quality requirements

Keep changes focused.

Reuse existing components where possible.

If a shared component becomes necessary, create it only when it reduces duplication.

Do not introduce unnecessary abstraction.

Do not add dead code.

Do not leave unused CSS classes.

Do not create new dependencies unless there is a strong reason.

## Verification

After changes, verify:

- `/login` renders normally
- `/register` renders normally
- `/about` renders normally
- `/profile` renders normally
- navigation labels are Chinese
- login/register visual style is consistent
- existing API calls are not changed
- fund list/search still works
- watchlist add/remove still works
- logout still works

## Output required from Codex

After finishing, report:

1. Files changed
2. What UI problems were fixed
3. Whether business logic was changed
4. How to verify locally
5. Any remaining known UI issues