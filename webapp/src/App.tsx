import { Suspense, lazy } from "react";
import { Route, Routes } from "react-router-dom";

import IndexPage from "@/pages/index";
const RandomPage = lazy(() => import("@/pages/random"));
const ArchivePage = lazy(() => import("@/pages/archive"));
const BlogPage = lazy(() => import("@/pages/statistics"));
const ZvideoPage = lazy(() => import("@/pages/zvideo"));
const AboutPage = lazy(() => import("@/pages/about"));

function App() {
	return (
		<Suspense fallback={<div>Loading...</div>}>
			<Routes>
				<Route path="/" element={<IndexPage />} />
				<Route path="/random" element={<RandomPage />} />
				<Route path="/archive" element={<ArchivePage />} />
				<Route path="/statistics" element={<BlogPage />} />
				<Route path="/zvideo" element={<ZvideoPage />} />
				<Route path="/about" element={<AboutPage />} />
			</Routes>
		</Suspense>
	);
}

export default App;
