import { Suspense, lazy } from "react";
import { Route, Routes } from "react-router-dom";

const IndexPage = lazy(() => import("@/pages/index"));
const RandomPage = lazy(() => import("@/pages/random"));
const ArchivePage = lazy(() => import("@/pages/archive"));
const BlogPage = lazy(() => import("@/pages/statistics"));
const ZvideoPage = lazy(() => import("@/pages/zvideo"));
const AboutPage = lazy(() => import("@/pages/about"));

function App() {
	return (
		<Routes>
			<Route
				path="/"
				element={
					<Suspense fallback={<div>Loading...</div>}>
						<IndexPage />
					</Suspense>
				}
			/>
			<Route
				path="/random"
				element={
					<Suspense fallback={<div>Loading...</div>}>
						<RandomPage />
					</Suspense>
				}
			/>
			<Route
				path="/archive"
				element={
					<Suspense fallback={<div>Loading...</div>}>
						<ArchivePage />
					</Suspense>
				}
			/>
			<Route
				path="/statistics"
				element={
					<Suspense fallback={<div>Loading...</div>}>
						<BlogPage />
					</Suspense>
				}
			/>
			<Route
				path="/zvideo"
				element={
					<Suspense fallback={<div>Loading...</div>}>
						<ZvideoPage />
					</Suspense>
				}
			/>
			<Route
				path="/about"
				element={
					<Suspense fallback={<div>Loading...</div>}>
						<AboutPage />
					</Suspense>
				}
			/>
		</Routes>
	);
}

export default App;
