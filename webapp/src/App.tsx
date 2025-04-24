import { Suspense, lazy } from "react";
import { Route, Routes } from "react-router-dom";

const IndexPage = lazy(() => import("@/pages/IndexPage"));
const RandomPage = lazy(() => import("@/pages/RandomPage"));
const ArchivePage = lazy(() => import("@/pages/ArchivePage"));
const BookmarkPage = lazy(() => import("@/pages/BookmarkPage"));
const StatisticsPage = lazy(() => import("@/pages/StatisticsPage"));
const ZvideoPage = lazy(() => import("@/pages/ZvideoPage"));
const AboutPage = lazy(() => import("@/pages/AboutPage"));

function App() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <Routes>
        <Route path="/" element={<IndexPage />} />
        <Route path="/random" element={<RandomPage />} />
        <Route path="/archive" element={<ArchivePage />} />
        <Route path="/bookmark" element={<BookmarkPage />} />
        <Route path="/statistics" element={<StatisticsPage />} />
        <Route path="/zvideo" element={<ZvideoPage />} />
        <Route path="/about" element={<AboutPage />} />
      </Routes>
    </Suspense>
  );
}

export default App;
