import React from 'react'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import SideBar from '@/components/SideBar'
import NavBar from '@/components/NavBar'
import { routes } from '@/routes/routes.jsx'
import './App.css'

function App() {
    return (
        <BrowserRouter>
            <div className="flex flex-col h-screen">
                {/* NavBar always on top */}
                <div className="bg-base-100 shadow">
                    <NavBar />
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
    )
}

export default App