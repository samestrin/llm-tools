import { Request, Response } from 'express';

interface UserDTO {
    id: number;
    name: string;
    email: string;
}

type UserRole = 'admin' | 'user' | 'guest';

export class UserController {
    private service: UserService;

    constructor(service: UserService) {
        this.service = service;
    }

    async getUser(req: Request, res: Response): Promise<void> {
        const user = await this.service.findById(req.params.id);
        res.json(user);
    }

    deleteUser(req: Request, res: Response): void {
        this.service.delete(req.params.id);
        res.status(204).send();
    }
}

export function createApp(): Express {
    const app = express();
    return app;
}

const handleError = (err: Error) => {
    console.error(err.message);
};

export const processData = async (data: UserDTO[]): Promise<UserDTO[]> => {
    return data.filter(u => u.id > 0);
};
