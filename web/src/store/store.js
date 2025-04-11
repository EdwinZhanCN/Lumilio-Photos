import { configureStore} from '@reduxjs/toolkit'
import addReducer from './addSlicer'

const store = configureStore({
    reducer: {
        add: addReducer,
    },
});

export default store;