import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import App from './App';
import Activities from './routes/activities';
import Profile from './routes/profile';
import Lab from './routes/lab';
import LabTrackDetail from './routes/lab-detail';
import LabCompare from './routes/lab-compare';
import './index.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<App />} />
        <Route path="/activities" element={<Activities />} />
        <Route path="/profile" element={<Profile />} />
        <Route path="/lab" element={<Lab />} />
        <Route path="/lab/:id" element={<LabTrackDetail />} />
        <Route path="/lab/compare" element={<LabCompare />} />
      </Routes>
    </BrowserRouter>
  </StrictMode>,
);
