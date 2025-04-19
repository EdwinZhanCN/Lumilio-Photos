import React, { useEffect } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import SideBar from '@/components/SideBar';
import NavBar from '@/components/NavBar';
import { routes } from '@/routes/routes'; // Updated import (removed .jsx extension)
import {
    QueryClient,
    QueryClientProvider,
} from '@tanstack/react-query';
import GlobalProvider from '@/contexts/GlobalContext'; // Updated import (removed .jsx extension)
import './App.css';
import Notifications from "@/components/Notifications"; // Updated import (removed .jsx extension)

// Create query client outside of component to avoid recreation on every render
const queryClient = new QueryClient();

function App(): React.ReactNode {
    // Typed the theme or provided default
    const theme: string = localStorage.getItem("theme") || "light";

    useEffect(() => {
        document.documentElement.setAttribute("data-theme", theme);
    }, [theme]); // Added theme to dependency array

    return (
        <GlobalProvider>
            <QueryClientProvider client={queryClient}>
                <BrowserRouter>
                    <div className="flex flex-col h-screen">
                        {/* NavBar always on top */}
                        <div className="bg-base-100 shadow">
                            <NavBar/>
                        </div>
                        {/* Main area with sidebar and content */}
                        <div className="flex flex-1 overflow-hidden">
                            {/* Sidebar */}
                            <div className="w-auto bg-base-200 shadow-lg">
                                <SideBar />
                            </div>
                            {/* Content */}
                            <div className="flex-1 p-4 overflow-y-auto">
                                <Routes>
                                    {routes.map((route) => (
                                        <Route
                                            key={route.path}
                                            path={route.path}
                                            element={route.element}
                                        />
                                    ))}
                                </Routes>
                            </div>
                        </div>
                    </div>
                </BrowserRouter>
            </QueryClientProvider>
            <Notifications/>
        </GlobalProvider>
    );
}

export default App;